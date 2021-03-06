package predictor

import (
	"errors"
	"fmt"
	"goline/solver"
	"goline/trainer"
	"goline/util"
	"math"
	"os"
	"sort"
	"strconv"
)

const (
	returnJson = "{\"returncode\":0,\"message\":[\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"],\"result\":[\"%s\"]}"
	errorjson  = "{\"returncode\":0,\"message\"{%s},\"result\":[]}"
	streamjson = "{\"returncode\":0,\"message\"{},\"result\":[%s]}"
)

func print_usage(argc int, argv []string) {
	log := util.GetLogger()
	log.Error(fmt.Sprintf("Usage:\n", ""))
	log.Error(fmt.Sprintf("\t%s job_name test_file model output_file threshold\n", argv[0]))
}

type tuple struct {
	first  float64
	second int
}

type tvector []tuple

func (dv tvector) Less(i, j int) bool {
	if dv[i].first < dv[j].first {
		return true
	}

	return false
}

func (dv tvector) Len() int {
	return len(dv)
}

func (dv tvector) Swap(i, j int) {
	var temp tuple = dv[i]
	dv[i] = dv[j]
	dv[j] = temp
}

func tied_rank(x []float64) []float64 {

	var sorted_x tvector = make([]tuple, len(x))
	for i := 0; i < len(x); i++ {
		sorted_x[i] = tuple{x[i], i}
	}

	sort.Sort(sorted_x)

	r := make([]float64, len(x))
	cur_val := sorted_x[0].first
	last_rank := 0
	for i := 0; i < len(sorted_x); i++ {
		if cur_val != sorted_x[i].first {
			cur_val = sorted_x[i].first
			for j := last_rank; j < i; j++ {
				r[sorted_x[j].second] = float64(last_rank+1+i) / 2.0
			}
			last_rank = i
		}

		if i == len(sorted_x)-1 {
			for j := last_rank; j < i+1; j++ {
				r[sorted_x[j].second] = float64(last_rank+i+2) / 2.0
			}
		}
	}

	return r
}

func auc(actual []float64, posterior []float64) float64 {
	log := util.GetLogger()
	r := tied_rank(posterior)
	num_positive := 0.
	sum_positive := 0.
	for i := 0; i < len(actual); i++ {
		if actual[i] == 1 {
			num_positive++
		}
	}

	num_negative := float64(len(actual)) - num_positive
	for i := 0; i < len(r); i++ {
		if actual[i] == 1 {
			sum_positive += r[i]
		}
	}

	if num_negative*num_positive < 0.00001 {
		log.Info(fmt.Sprintf("num_positive %d, num_negative %d, sum_positive%d\n", num_positive, num_negative, sum_positive))
		return 0.
	}

	auc := ((sum_positive - num_positive*(num_positive+1)/2.0) /
		(num_negative * num_positive))

	return auc
}

func calc_auc(scores util.Dvector) float64 {
	var label []float64
	var predict []float64

	for i := 0; i < len(scores); i++ {
		label = append(label, scores[i].Second)
		predict = append(predict, scores[i].First)
	}

	auc := auc(label, predict)
	return auc
}

func Run(argc int, argv []string) (string, error) {

	var job_name string
	var test_file string
	var model_file string
	var output_file string
	var threshold float64
	log := util.GetLogger()

	if len(argv) == 5 {
		job_name = argv[0]
		test_file = argv[1]
		model_file = argv[2]
		output_file = argv[3]
		threshold, _ = strconv.ParseFloat(argv[4], 64)
	} else {
		print_usage(argc, argv)
		log.Error("[Predictor-Run] Input parameters error.")
		return fmt.Sprintf(errorjson, "[Predictor-Run] Input parameters error."), errors.New("[Predictor-Run] Input parameters error.")
	}

	if len(job_name) == 0 || len(test_file) == 0 || len(model_file) == 0 || len(output_file) == 0 {
		print_usage(argc, argv)
		log.Error("[Predictor-Run] Input parameters error.")
		return fmt.Sprintf(errorjson, "[Predictor-Run] Input parameters error."), errors.New("[Predictor-Run] Input parameters error.")
	}

	var model solver.LRModel
	model.Initialize(model_file)

	var wfp *os.File
	var err1 error
	exist := func(filename string) bool {
		var exist = true
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			exist = false
		}
		return exist
	}

	if exist(output_file) {
		wfp, err1 = os.OpenFile(output_file, os.O_SYNC, 0666)
	} else {
		wfp, err1 = os.Create(output_file)
	}

	if err1 != nil {
		log.Error("[Predictor-Run] Open file error." + err1.Error())
		return fmt.Sprintf(errorjson, err1.Error()), errors.New("[Predictor-Run] Open file error." + err1.Error())
	}

	defer wfp.Close()

	cnt := 0      //样本总数
	pcorrect := 0 //正样本预测正确数
	pcnt := 0     //正样本总数
	ncorrect := 0 //负样本预测正确数
	var loss float64 = 0.
	var parser trainer.FileParser
	err := parser.OpenFile(test_file)
	if err != nil {
		log.Error("[Predictor-Run] Open file error." + err.Error())
		return fmt.Sprintf(errorjson, err.Error()), errors.New("[Predictor-Run] Open file error." + err.Error())
	}

	var pred_scores util.Dvector

	for {
		res, y, x := parser.ReadSample()
		if res != nil {
			break
		}

		pred := model.Predict(x)
		pred = math.Max(math.Min(pred, 1.-10e-15), 10e-15)
		wfp.WriteString(fmt.Sprintf("%f\n", pred))

		pred_scores = append(pred_scores, util.DPair{pred, y})

		cnt++
		if util.UtilFloat64Equal(y, 1.0) {
			pcnt++
		}

		var pred_label float64 = 0
		if pred > threshold {
			pred_label = 1
		}

		if util.UtilFloat64Equal(pred_label, y) {
			if util.UtilFloat64Equal(y, 1.0) {
				pcorrect++
			} else {
				ncorrect++
			}
		}

		pred = math.Max(math.Min(pred, 1.-10e-15), 10e-15)
		if y > 0 {
			loss += -math.Log(pred)
		} else {
			loss += -math.Log(1. - pred)
		}

	}

	auc := calc_auc(pred_scores)
	if auc < 0.5 {
		auc = 0.5
	}

	if cnt > 0 {
		log.Info(fmt.Sprintf("[%s] Log-likelihood = %f\n", job_name, float64(loss)/float64(cnt)))
		log.Info(fmt.Sprintf("[%s] Precision = %.2f%% (%d/%d)\n", job_name,
			float64(pcorrect*100)/float64(cnt-pcnt-ncorrect+pcorrect),
			pcorrect, cnt-pcnt-ncorrect+pcorrect))
		log.Info(fmt.Sprintf("[%s] Recall = %.2f%% (%d/%d)\n", job_name,
			float64(pcorrect*100)/float64(pcnt), pcorrect, pcnt))
		log.Info(fmt.Sprintf("[%s] Accuracy = %.2f%% (%d/%d)\n", job_name,
			float64((pcorrect+ncorrect)*100)/float64(cnt), (pcorrect + ncorrect), cnt))
		log.Info(fmt.Sprintf("[%s] AUC = %f\n", job_name, auc))
	}

	parser.CloseFile()

	util.Write2File(output_file, fmt.Sprintf(" Log-likelihood = %f\n Precision = %f (%d/%d)\n Recall = %f (%d/%d)\n Accuracy = %f (%d/%d)\n AUC = %f\n",
		float64(loss)/float64(cnt),
		float64(pcorrect)/float64(cnt-pcnt-ncorrect+pcorrect), pcorrect, cnt-pcnt-ncorrect+pcorrect,
		float64(pcorrect)/float64(pcnt), pcorrect, pcnt,
		float64(pcorrect+ncorrect)/float64(cnt), pcorrect+ncorrect, cnt,
		auc))

	return fmt.Sprintf(returnJson,
		job_name,
		fmt.Sprintf("Log-likelihood = %f", float64(loss)/float64(cnt)),
		fmt.Sprintf("Precision = %f (%d/%d)", float64(pcorrect)/float64(cnt-pcnt-ncorrect+pcorrect), pcorrect, cnt-pcnt-ncorrect+pcorrect),
		fmt.Sprintf("Recall = %f (%d/%d)", float64(pcorrect)/float64(pcnt), pcorrect, pcnt),
		fmt.Sprintf("Accuracy = %f (%d/%d)", float64((pcorrect+ncorrect))/float64(cnt), (pcorrect+ncorrect), cnt),
		fmt.Sprintf("AUC = %f", auc),
		output_file), nil
}

func StreamRun(model_file string, instances []string) (string, error) {
	log := util.GetLogger()
	if !util.FileExists(model_file) || len(instances) == 0 {
		log.Error("[Predictor-StreamRun] Model file or instances error.")
		return fmt.Sprintf(errorjson, "[Predictor-StreamRun] Model file or instances error."), errors.New("[Predictor-StreamRun] Model file or instances error.")
	}

	var rtstr string
	var model solver.LRModel
	model.Initialize(model_file)
	for i := 0; i < len(instances); i++ {
		res, _, x := util.ParseSample(instances[i])
		if res != nil {
			break
		}

		pred := model.Predict(x)
		pred = math.Max(math.Min(pred, 1.-10e-15), 10e-15)
		if i == len(instances)-1 {
			rtstr += strconv.FormatFloat(pred, 'f', 6, 64)
		} else {
			rtstr += strconv.FormatFloat(pred, 'f', 6, 64) + ","
		}
	}

	return fmt.Sprintf(streamjson, rtstr), nil
}
