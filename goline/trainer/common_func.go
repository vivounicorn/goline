package trainer

import (
	"bufio"
	"errors"
	"fmt"
	"goline/util"
	"io"
	"math"
	"os"
	"strconv"
	s "strings"
	"sync"
)

func calc_loss(y float64, pred float64) float64 {
	max_sigmoid := util.MaxSigmoid
	min_sigmoid := util.MinSigmoid
	one := 1.
	pred = math.Max(math.Min(pred, max_sigmoid), min_sigmoid)
	var loss float64
	if y > 0 {
		loss = -math.Log(pred)
	} else {
		loss = -math.Log(one - pred)
	}

	return loss
}

func read_problem_info(
	train_file string,
	read_cache bool,
	num_threads int) (int, int, error) {

	feat_num := 0
	line_cnt := 0

	log := util.GetLogger()

	var lock sync.Mutex
	var parser FileParser
	var errall error

	read_from_cache := func(path string) error {
		fs, err := os.Open(path)
		defer fs.Close()
		if err != nil {
			return err
		}

		bfRd := bufio.NewReader(fs)
		line, err := bfRd.ReadString('\n')
		if err != nil {
			return err
		}

		var res []string = s.Split(line, " ")
		if len(res) != 2 {
			log.Error("[read_problem_info] File format error.")
			return errors.New("[read_problem_info] File format error.")
		}

		feat_num, errall = strconv.Atoi(res[0])
		if errall != nil {
			log.Error("[read_problem_info] Label format error." + errall.Error())
			return errors.New("[read_problem_info] Label format error." + errall.Error())
		}
		line_cnt, errall = strconv.Atoi(res[1])
		if errall != nil {
			log.Error("[read_problem_info] Feature format error." + errall.Error())
			return errors.New("[read_problem_info] Feature format error." + errall.Error())
		}

		return nil
	}

	exist := func(filename string) bool {
		var exist = true
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			exist = false
		}
		return exist
	}

	write_to_cache := func(filename string) error {
		var f *os.File
		var err1 error
		if exist(filename) {
			f, err1 = os.OpenFile(filename, os.O_WRONLY, 0666)
		} else {
			f, err1 = os.Create(filename)
		}

		if err1 != nil {
			return err1
		}

		defer f.Close()

		wireteString := string(feat_num) + " " + string(line_cnt) + "\n"
		_, err1 = io.WriteString(f, wireteString)
		if err1 != nil {
			return err1
		}
		return nil
	}

	read_problem_worker := func(i int, c *sync.WaitGroup) {
		local_max_feat := 0
		local_count := 0
		for {
			flag, _, local_x := parser.ReadSampleMultiThread()
			if flag != nil {
				break
			}

			for i := 0; i < len(local_x); i++ {
				if local_x[i].Index+1 > local_max_feat {
					local_max_feat = local_x[i].Index + 1
				}
			}
			local_count++
		}

		lock.Lock()
		line_cnt += local_count
		lock.Unlock()
		if local_max_feat > feat_num {
			feat_num = local_max_feat
		}

		defer c.Done()
	}

	cache_file := string(train_file) + ".cache"
	cache_exists := exist(cache_file)
	if read_cache && cache_exists {
		read_from_cache(cache_file)
	} else {
		parser.OpenFile(train_file)
		util.UtilParallelRun(read_problem_worker, num_threads)
		parser.CloseFile()
	}

	log.Info(fmt.Sprintf("[read_problem_info] Instances=[%d] features=[%d]\n", line_cnt, feat_num))

	if read_cache && !cache_exists {
		write_to_cache(cache_file)
	}

	return feat_num, line_cnt, nil
}

func evaluate_file(path string, func_predict func(x util.Pvector) float64, num_threads int) float64 {
	var parser FileParser
	parser.OpenFile(path)

	count := 0
	var loss float64 = 0
	var lock sync.Mutex
	var predict_worker = func(i int, c *sync.WaitGroup) {

		local_count := 0
		var local_loss float64 = 0
		for {
			res, local_y, local_x := parser.ReadSampleMultiThread()
			if res != nil {
				break
			}

			local_loss += calc_loss(local_y, func_predict(local_x))
			local_count++
		}

		lock.Lock()
		count += local_count
		loss += local_loss
		lock.Unlock()

		defer c.Done()
	}

	util.UtilParallelRun(predict_worker, num_threads)

	parser.CloseFile()
	if count > 0 {
		loss = loss / float64(count)
	}

	return loss
}

func evaluate_stream(stream []string, func_predict func(x util.Pvector) float64, num_threads int) float64 {
	var parser StreamParser
	parser.Open(stream)

	count := 0
	var loss float64 = 0
	var lock sync.Mutex
	var predict_worker = func(i int, c *sync.WaitGroup) {

		local_count := 0
		var local_loss float64 = 0
		for {
			res, local_y, local_x := parser.ReadSampleMultiThread()
			if res != nil {
				break
			}

			local_loss += calc_loss(local_y, func_predict(local_x))
			local_count++
		}

		lock.Lock()
		count += local_count
		loss += local_loss
		lock.Unlock()

		defer c.Done()
	}

	util.UtilParallelRun(predict_worker, num_threads)

	parser.Close()
	if count > 0 {
		loss = loss / float64(count)
	}

	return loss
}
