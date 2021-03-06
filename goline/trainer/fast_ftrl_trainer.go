package trainer

import (
	"errors"
	"fmt"
	"goline/deps/log4go"
	"goline/solver"
	"goline/util"
	"math"
	"runtime"
	"sync"
)

type FastFtrlTrainer struct {
	Epoch           int
	CacheFeatureNum bool
	PusStep         int
	FetchStep       int
	BurnIn          float64

	ParamServer solver.FtrlParamServer
	NumThreads  int

	JobName string

	Init    bool
	log4fft log4go.Logger
}

func (fft *FastFtrlTrainer) SetJobName(name string) {

	fft.JobName = "fastftrljob"
	if name != "" {
		fft.JobName = name
	}
}

func (fft *FastFtrlTrainer) Initialize(
	epoch int,
	num_threads int,
	cache_feature_num bool,
	burn_in float64,
	push_step int,
	fetch_step int) bool {
	fft.Epoch = epoch
	fft.CacheFeatureNum = cache_feature_num
	fft.PusStep = push_step
	fft.FetchStep = fetch_step
	if num_threads == 0 {
		fft.NumThreads = runtime.NumCPU()
	} else {
		fft.NumThreads = num_threads
	}

	fft.Init = true
	fft.BurnIn = burn_in
	fft.log4fft = util.GetLogger()
	return fft.Init
}

func (fft *FastFtrlTrainer) Train(
	alpha float64,
	beta float64,
	l1 float64,
	l2 float64,
	dropout float64,
	model_file string,
	train_file string,
	test_file string) error {

	if !fft.Init {
		fft.log4fft.Error("[FastFtrlTrainer-Train] Fast ftrl trainer initialize error.")
		return errors.New("[FastFtrlTrainer-Train] Fast ftrl trainer initialize error.")
	}

	if !util.FileExists(train_file) || !util.FileExists(test_file) {
		fft.log4fft.Error("[FastFtrlTrainer-Train] Train file or test file is not exist.")
		return errors.New("[FastFtrlTrainer-Train] Train file or test file is not exist.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, fft.CacheFeatureNum, fft.NumThreads)
	if feat_num == 0 {
		fft.log4fft.Error("[FastFtrlTrainer-Train] The number of features is zero.")
		return errors.New("[FastFtrlTrainer-Train] The number of features is zero.")
	}

	err := fft.ParamServer.Initialize(alpha, beta, l1, l2, feat_num, dropout)
	if err != nil {
		fft.log4fft.Error(fmt.Sprintf("[FastFtrlTrainer-Train] Parameter server initializing error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[FastFtrlTrainer-Train] Parameter server initializing error.%s", err.Error()))
	}

	return fft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (fft *FastFtrlTrainer) TrainRestore(
	last_model string,
	model_file string,
	train_file string,
	test_file string) error {

	if !fft.Init {
		fft.log4fft.Error("[FastFtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
		return errors.New("[FastFtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, fft.CacheFeatureNum, fft.NumThreads)
	if feat_num == 0 {
		fft.log4fft.Error("[FastFtrlTrainer-TrainRestore] The number of features is zero.")
		return errors.New("[FastFtrlTrainer-TrainRestore] The number of features is zero.")
	}

	err := fft.ParamServer.Construct(last_model)
	if err != nil {
		fft.log4fft.Error(fmt.Sprintf("[FastFtrlTrainer-TrainRestore] Parameter server restore error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[FastFtrlTrainer-TrainRestore] Parameter server restore error.%s", err.Error()))
	}

	return fft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (fft *FastFtrlTrainer) TrainImpl(
	model_file string,
	train_file string,
	line_cnt int,
	test_file string) error {

	if !fft.Init {
		fft.log4fft.Error("[FastFtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
		return errors.New("[FastFtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
	}

	fft.log4fft.Info(fmt.Sprintf(
		"[%s] params={alpha:%.2f, beta:%.2f, l1:%.2f, l2:%.2f, dropout:%.2f, epoch:%d}\n",
		fft.JobName,
		fft.ParamServer.Alpha,
		fft.ParamServer.Beta,
		fft.ParamServer.L1,
		fft.ParamServer.L2,
		fft.ParamServer.Dropout,
		fft.Epoch))

	var solvers []solver.FtrlWorker = make([]solver.FtrlWorker, fft.NumThreads)
	for i := 0; i < fft.NumThreads; i++ {
		solvers[i].Initialize(&fft.ParamServer, fft.PusStep, fft.FetchStep)
	}

	predict_func := func(x util.Pvector) float64 {
		return fft.ParamServer.Predict(x)
	}

	var timer util.StopWatch
	timer.StartTimer()
	for iter := 0; iter < fft.Epoch; iter++ {
		var file_parser ParallelFileParser
		file_parser.OpenFile(train_file, fft.NumThreads)
		count := 0
		var loss float64 = 0.

		var lock sync.Mutex
		worker_func := func(i int, c *sync.WaitGroup) {
			local_count := 0
			var local_loss float64 = 0
			for {
				flag, y, x := file_parser.ReadSampleMultiThread(i)
				if flag != nil {
					break
				}

				pred := solvers[i].Update(x, y, &fft.ParamServer)
				local_loss += calc_loss(y, pred)
				local_count++

				if i == 0 && local_count%10000 == 0 {
					tmp_cnt := math.Min(float64(local_count*fft.NumThreads), float64(line_cnt))
					fft.log4fft.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
						fft.JobName,
						iter,
						float64(tmp_cnt*100)/float64(line_cnt),
						timer.StopTimer(),
						float64(local_loss)/float64(local_count)))
				}
			}
			lock.Lock()
			count += local_count
			loss += local_loss
			lock.Unlock()

			solvers[i].PushParam(&fft.ParamServer)
			defer c.Done()
		}

		if iter == 0 && util.UtilGreater(fft.BurnIn, float64(0)) {
			burn_in_cnt := int(fft.BurnIn * float64(line_cnt))
			var local_loss float64 = 0
			for i := 0; i < burn_in_cnt; i++ {
				//线程0做预热
				flag, y, x := file_parser.ReadSample(0)
				if flag != nil {
					break
				}

				pred := fft.ParamServer.Update(x, y)
				local_loss += calc_loss(y, pred)
				if i%10000 == 0 {
					fft.log4fft.Info(fmt.Sprintf("[%s] burn-in processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
						fft.JobName,
						float64((i+1)*100)/float64(line_cnt),
						timer.StopTimer(),
						float64(local_loss)/float64(i+1)))
				}
			}

			fft.log4fft.Info(fmt.Sprintf("[%s] burn-in processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
				fft.JobName,
				float64(burn_in_cnt*100)/float64(line_cnt),
				timer.StopTimer(),
				float64(local_loss)/float64(burn_in_cnt)))

			if util.UtilFloat64Equal(fft.BurnIn, float64(1)) {
				continue
			}
		}

		for i := 0; i < fft.NumThreads; i++ {
			solvers[i].Reset(&fft.ParamServer)
		}

		util.UtilParallelRun(worker_func, fft.NumThreads)

		file_parser.CloseFile(fft.NumThreads)

		//		f(w,
		//			"[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
		//			fft.JobName,
		//			iter,
		//			float64(count*100)/float64(line_cnt),
		//			timer.StopTimer(),
		//			float64(loss)/float64(count))

		if test_file != "" {
			eval_loss := evaluate_file(test_file, predict_func, fft.NumThreads)
			fft.log4fft.Info(fmt.Sprintf("[%s] validation-loss=[%f]\n", fft.JobName, float64(eval_loss)))
		}
	}

	return fft.ParamServer.SaveModel(model_file)
}
