package trainer

import (
	"errors"
	"fmt"
	"goline/deps/log4go"
	"goline/solver"
	"goline/util"
)

type FtrlTrainer struct {
	Epoch           int
	CacheFeatureNum bool
	Solver          solver.FtrlSolver
	Init            bool
	JobName         string
	log             log4go.Logger
}

func (ft *FtrlTrainer) SetJobName(name string) {

	ft.JobName = "ftrljob"
	if name != "" {
		ft.JobName = name
	}
}

func (ft *FtrlTrainer) Initialize(epoch int, cache_feature_num bool) bool {
	ft.Epoch = epoch
	ft.CacheFeatureNum = cache_feature_num
	ft.Init = true
	ft.log = util.GetLogger()
	return ft.Init
}

func (ft *FtrlTrainer) Train(
	alpha float64,
	beta float64,
	l1 float64,
	l2 float64,
	dropout float64,
	model_file string,
	train_file string,
	test_file string) error {

	if !ft.Init {
		ft.log.Error("[FtrlTrainer-Train] Fast ftrl trainer initialize error.")
		return errors.New("[FtrlTrainer-Train] Fast ftrl trainer initialize error.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, ft.CacheFeatureNum, 0)
	if feat_num == 0 {
		ft.log.Error("[FtrlTrainer-Train] The number of features is zero.")
		return errors.New("[FtrlTrainer-Train] The number of features is zero.")
	}

	if !ft.Solver.Initialize(alpha, beta, l1, l2, feat_num, dropout) {
		ft.log.Error("[FtrlTrainer-Train] Solver initializing error.")
		return errors.New("[FtrlTrainer-Train] Solver initializing error.")
	}

	return ft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (ft *FtrlTrainer) TrainRestore(
	last_model string,
	model_file string,
	train_file string,
	test_file string) error {
	if !ft.Init {
		ft.log.Error("[FtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
		return errors.New("[FtrlTrainer-TrainRestore] Fast ftrl trainer restore error.")
	}

	feat_num, line_cnt, _ := read_problem_info(train_file, ft.CacheFeatureNum, 0)
	if feat_num == 0 {
		ft.log.Error("[FtrlTrainer-TrainRestore] The number of features is zero.")
		return errors.New("[FtrlTrainer-TrainRestore] The number of features is zero.")
	}

	err := ft.Solver.Construct(last_model)
	if err != nil {
		ft.log.Error(fmt.Sprintf("[FtrlTrainer-TrainRestore] Solver restore error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[FtrlTrainer-TrainRestore] Solver restore error.%s", err.Error()))
	}

	return ft.TrainImpl(model_file, train_file, line_cnt, test_file)
}

func (ft *FtrlTrainer) TrainImpl(
	model_file string,
	train_file string,
	line_cnt int,
	test_file string) error {
	if !ft.Init {
		ft.log.Error("[FtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
		return errors.New("[FtrlTrainer-TrainImpl] Fast ftrl trainer restore error.")
	}

	ft.log.Info(fmt.Sprintf("[%s] params={alpha:%.2f, beta:%.2f, l1:%.2f, l2:%.2f, dropout:%.2f, epoch:%d}\n",
		ft.JobName,
		ft.Solver.Alpha,
		ft.Solver.Beta,
		ft.Solver.L1,
		ft.Solver.L2,
		ft.Solver.Dropout,
		ft.Epoch))

	predict_func := func(x util.Pvector) float64 {
		return ft.Solver.Predict(x)
	}

	var timer util.StopWatch
	timer.StartTimer()
	var last_time float64 = 0
	for iter := 0; iter < ft.Epoch; iter++ {
		var file_parser FileParser
		file_parser.OpenFile(train_file)

		cur_cnt := 0
		last_cnt := 0
		var loss float64 = 0
		for {
			flag, y, x := file_parser.ReadSample()
			if flag != nil {
				break
			}

			pred := ft.Solver.Update(x, y)
			loss += calc_loss(y, pred)
			cur_cnt++

			if cur_cnt-last_cnt > 100000 && timer.StopTimer()-last_time > 0.5 {
				ft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
					ft.JobName,
					iter,
					float64(cur_cnt*100)/float64(line_cnt),
					timer.StopTimer(),
					float64(loss)/float64(cur_cnt)))

				last_cnt = cur_cnt
				last_time = timer.StopTimer()
			}
		}
		ft.log.Info(fmt.Sprintf("[%s] epoch=%d processed=[%.2f%%] time=[%.2f] train-loss=[%.6f]\n",
			ft.JobName,
			iter,
			float64(cur_cnt*100)/float64(line_cnt),
			timer.StopTimer(),
			float64(loss)/float64(cur_cnt)))

		file_parser.CloseFile()

		if test_file != "" {
			eval_loss := evaluate_file(test_file, predict_func, 0)
			ft.log.Info(fmt.Sprintf("[%s] validation-loss=[%f]\n", float64(eval_loss)))
		}
	}

	return ft.Solver.SaveModel(model_file)
}
