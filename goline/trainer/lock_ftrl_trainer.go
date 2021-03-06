package trainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"goline/deps/log4go"
	"goline/solver"
	"goline/util"
	"io"
	"math"
	"sync"
)

type LockFreeFtrlTrainer struct {
	Epoch           int
	CacheFeatureNum bool
	Solver          solver.FtrlSolver
	Init            bool
	NumThreads      int
	JobName         string
	log             log4go.Logger
}

func (lft *LockFreeFtrlTrainer) SetJobName(name string) {

	lft.JobName = "lockftrljob"
	if name != "" {
		lft.JobName = name
	}
}

func (lft *LockFreeFtrlTrainer) Initialize(
	epoch int,
	num_threads int,
	cache_feature_num bool) bool {
	lft.Epoch = epoch
	lft.CacheFeatureNum = cache_feature_num
	lft.NumThreads = num_threads
	lft.log = util.GetLogger()

	lft.Init = true
	return lft.Init
}

func (lft *LockFreeFtrlTrainer) Train(
	alpha float64,
	beta float64,
	l1 float64,
	l2 float64,
	dropout float64,
	model_file string,
	train_file string,
	test_file string) error {

	if !lft.Init {
		lft.log.Error("[LockFreeFtrlTrainer-Train] Fast ftrl trainer initialize error.")
		return errors.New("[LockFreeFtrlTrainer-Train] Fast ftrl trainer initialize error.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, lft.CacheFeatureNum, lft.NumThreads)
	if feat_num == 0 {
		lft.log.Error("[LockFreeFtrlTrainer-Train] The number of features is zero.")
		return errors.New("[LockFreeFtrlTrainer-Train] The number of features is zero.")
	}

	if !lft.Solver.Initialize(alpha, beta, l1, l2, feat_num, dropout) {
		lft.log.Info("[LockFreeFtrlTrainer-Train] Solver initializing error.")
		return errors.New("[LockFreeFtrlTrainer-Train] Solver initializing error.")
	}

	return lft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (lft *LockFreeFtrlTrainer) TrainRestore(
	last_model string,
	model_file string,
	train_file string,
	test_file string) error {
	if !lft.Init {
		lft.log.Error("[LockFreeFtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
		return errors.New("[LockFreeFtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, lft.CacheFeatureNum, lft.NumThreads)
	if feat_num == 0 {
		lft.log.Error("[LockFreeFtrlTrainer-TrainRestore] The number of features is zero.")
		return errors.New("[LockFreeFtrlTrainer-TrainRestore] The number of features is zero.")
	}

	err := lft.Solver.Construct(last_model)
	if err != nil {
		lft.log.Error(fmt.Sprintf("[LockFreeFtrlTrainer-TrainRestore] Solver restore error.", err.Error()))
		return errors.New(fmt.Sprintf("[LockFreeFtrlTrainer-TrainRestore] Solver restore error.", err.Error()))
	}

	return lft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (lft *LockFreeFtrlTrainer) TrainImpl(
	model_file string,
	train_file string,
	line_cnt int,
	test_file string) error {
	if !lft.Init {
		lft.log.Error("[LockFreeFtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
		return errors.New("[LockFreeFtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
	}

	lft.log.Info(fmt.Sprintf("[%s] params={alpha:%.2f, beta:%.2f, l1:%.2f, l2:%.2f, dropout:%.2f, epoch:%d}\n",
		lft.JobName,
		lft.Solver.Alpha,
		lft.Solver.Beta,
		lft.Solver.L1,
		lft.Solver.L2,
		lft.Solver.Dropout,
		lft.Epoch))

	predict_func := func(x util.Pvector) float64 {
		return lft.Solver.Predict(x)
	}

	var timer util.StopWatch
	timer.StartTimer()
	for iter := 0; iter < lft.Epoch; iter++ {
		var file_parser FileParser
		file_parser.OpenFile(train_file)

		count := 0
		var loss float64 = 0

		var lock sync.Mutex

		worker_func := func(i int, c *sync.WaitGroup) {
			local_count := 0
			var local_loss float64 = 0
			for {
				flag, y, x := file_parser.ReadSampleMultiThread()
				if flag != nil {
					break
				}

				pred := lft.Solver.Update(x, y)
				local_loss += calc_loss(y, pred)
				local_count++

				if i == 0 && local_count%10000 == 0 {
					tmp_cnt := math.Min(float64(local_count*lft.NumThreads), float64(line_cnt))
					lft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
						lft.JobName,
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
			defer c.Done()
		}

		util.UtilParallelRun(worker_func, lft.NumThreads)

		file_parser.CloseFile()

		lft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
			lft.JobName,
			iter,
			float64(count*100)/float64(line_cnt),
			timer.StopTimer(),
			float64(loss)/float64(count)))

		if test_file != "" {
			eval_loss := evaluate_file(test_file, predict_func, 0)
			lft.log.Info(fmt.Sprintf("[%s] validation-loss=[%f]\n", lft.JobName, float64(eval_loss)))
		}
	}

	return lft.Solver.SaveModel(model_file)
}

func (lft *LockFreeFtrlTrainer) TrainBatch(
	encodemodel string,
	instances []string) error {

	line_cnt := len(instances)
	if line_cnt == 0 {
		lft.log.Error("[LockFreeFtrlTrainer-TrainBatch] No model retrained.")
		return errors.New("[LockFreeFtrlTrainer-TrainBatch] No model retrained.")
	}

	var fls solver.FtrlSolver
	err := json.Unmarshal([]byte(encodemodel), &fls)
	if err != nil {
		lft.log.Error("[LockFreeFtrlTrainer-TrainBatch]" + err.Error())
		return errors.New("[LockFreeFtrlTrainer-TrainBatch]" + err.Error())
	}

	lft.Solver = fls

	lft.log.Info(fmt.Sprintf("[%s] params={alpha:%.2f, beta:%.2f, l1:%.2f, l2:%.2f, dropout:%.2f, epoch:%d}\n",
		lft.JobName,
		lft.Solver.Alpha,
		lft.Solver.Beta,
		lft.Solver.L1,
		lft.Solver.L2,
		lft.Solver.Dropout,
		lft.Epoch))

	predict_func := func(x util.Pvector) float64 {
		return lft.Solver.Predict(x)
	}

	var timer util.StopWatch
	timer.StartTimer()
	for iter := 0; iter < lft.Epoch; iter++ {
		var stream_parser StreamParser
		stream_parser.Open(instances)

		count := 0
		var loss float64 = 0

		var lock sync.Mutex

		worker_func := func(i int, c *sync.WaitGroup) {
			local_count := 0
			var local_loss float64 = 0
			for {
				flag, y, x := stream_parser.ReadSampleMultiThread()
				if flag != nil {
					break
				}

				pred := lft.Solver.Update(x, y)
				local_loss += calc_loss(y, pred)
				local_count++

				if i == 0 && local_count%10000 == 0 {
					tmp_cnt := math.Min(float64(local_count*lft.NumThreads), float64(line_cnt))
					lft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
						lft.JobName,
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
			defer c.Done()
		}

		util.UtilParallelRun(worker_func, lft.NumThreads)

		stream_parser.Close()

		lft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
			lft.JobName,
			iter,
			float64(count*100)/float64(line_cnt),
			timer.StopTimer(),
			float64(loss)/float64(count)))

		eval_loss := evaluate_stream(instances, predict_func, 0)
		lft.log.Info(fmt.Sprintf("[%s] validation-loss=[%f]\n", lft.JobName, float64(eval_loss)))
	}

	return nil
}

func (lft *LockFreeFtrlTrainer) TrainOnline(
	encodemodel string,
	instances []string) (string, error) {

	err := lft.TrainBatch(encodemodel, instances)
	if err != nil {
		lft.log.Error("[LockFreeFtrlTrainer-TrainOnline] Online learning failed." + err.Error())
		return encodemodel, errors.New("[LockFreeFtrlTrainer-TrainOnline] Online learning failed." + err.Error())
	}

	return lft.Solver.SaveEncodeModel()
}

func (lft *LockFreeFtrlTrainer) TrainOnlineAndDump(
	w io.Writer,
	encodemodel string,
	instances []string,
	path string,
	f func(w io.Writer, format string, a ...interface{}) (int, error)) error {

	err := lft.TrainBatch(encodemodel, instances)
	if err != nil {
		lft.log.Error(fmt.Sprintf("[LockFreeFtrlTrainer-TrainOnlineAndDump] Online learning failed.", err.Error()))
		return errors.New(fmt.Sprintf("[LockFreeFtrlTrainer-TrainOnlineAndDump] Online learning failed.", err.Error()))
	}

	return lft.Solver.SaveModel(path)
}
