package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"goline/deps/log4go"
	"goline/hdfs"
	"goline/predictor"
	"goline/trainer"
	"goline/util"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	s "strings"
	"time"
)

type Lands struct {
	pool       *redis.Pool
	mux        map[string]func(http.ResponseWriter, *util.ModelParam) error
	conf       *util.Config
	log4goline log4go.Logger
}

const (
	JsonError        = "{\"returncode\": 1,\"message\": \"%s\",\"result\": []}"
	TimeFormatString = "200601021504"
	ModelPrefix      = "md_"
)

func (lan *Lands) Initialize(configFile string) error {

	var err error
	lan.mux = make(map[string]func(http.ResponseWriter, *util.ModelParam) error)
	lan.mux["/goline/online"] = lan.onlineServeHttp
	lan.mux["/goline/offline"] = lan.offlineServeHttp
	lan.mux["/goline/predict"] = lan.predictServeHttp

	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		lan.log4goline.Error("[Lands-Initialize]Open Config error." + err.Error())
		return errors.New("[Lands-Initialize]Open Config error." + err.Error())
	}

	temp := new(util.Config)
	if err = json.Unmarshal(file, temp); err != nil {
		lan.log4goline.Error("[Lands-Initialize]Parse config file error." + err.Error())
		return errors.New("[Lands-Initialize]Parse config file error." + err.Error())
	}

	lan.conf = temp

	lan.pool = util.InitRedisPool(&lan.conf.Redis)
	if lan.pool == nil {
		lan.log4goline.Error("[Lands-Initialize]Initialize redis pool error.")
		return errors.New("[Lands-Initialize]Initialize redis pool error.")
	}

	util.InitLogger(lan.conf.LogModule)
	lan.log4goline = util.GetLogger()

	return nil
}

func (lan *Lands) createHdfsClient() (*hdfs.Client, error) {
	var err error
	var client *hdfs.Client
	namenodes := strings.Split(lan.conf.NameNodes, ",")
	for i := range namenodes {
		client, err = hdfs.NewForUser(namenodes[i]+":8020", lan.conf.HadoopUser)
		if err == nil {
			break
		}
	}

	if err != nil {
		lan.log4goline.Error("[Lands-Initialize]Create hdfs client error." + err.Error())
		return nil, errors.New("[Lands-Initialize]Create hdfs client error." + err.Error())
	}

	return client, nil
}

func (lan *Lands) checkData(filename string) (int64, error) {
	var count int64 = 0
	fs, err := os.Open(filename)
	if err != nil {
		lan.log4goline.Error("[Lands-CheckData] Open file failed." + err.Error())
		return 0, errors.New("[Lands-CheckData] Open file failed." + err.Error())
	}

	defer fs.Close()

	buf := bufio.NewReader(fs)

	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return count, nil
			} else {
				lan.log4goline.Error("[Lands-CheckData] Open file failed." + err.Error())
				return 0, errors.New("[Lands-CheckData] Open file failed." + err.Error())
			}
		}
		line = s.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		sp := s.Split(line, lan.conf.SampleSpliter)
		count++
		if len(sp) <= 1 {
			str := fmt.Sprintf("[Lands-CheckData] (spliter must be space) file %s,line %d,content %s", filename, count, line)
			lan.log4goline.Error(str)
			return 0, errors.New(fmt.Sprintf(JsonError, str)) //分隔符错误(不是space)
		}

		var start int
		var length int
		if s.Contains(line, "|f") {
			start = 3
			length = len(sp) - 3
		} else {
			start = 1
			length = len(sp)
		}

		label, err := strconv.ParseFloat(sp[0], 64)
		if err != nil || (int(label) != -1 && int(label) != 0 && int(label) != 1) {
			str := fmt.Sprintf("[Lands-CheckData] (label must be -1,0/1) file %s,line %d,content %s", filename, count, label)
			lan.log4goline.Error(str)
			return 0, errors.New(fmt.Sprintf(JsonError, str)) //标注错误(不是0或1)
		}

		for i := start; i < length; i++ {
			tup := s.Split(sp[i], ":")
			if len(tup) != 2 {
				str := fmt.Sprintf("[Lands-CheckData] (feature value must be key:value) file %s,line %d,content %s", filename, count, sp[i])
				lan.log4goline.Error(str)
				return 0, errors.New(fmt.Sprintf(JsonError, str)) //特征格式错误(不是key:value)
			}
		}
	}

	return count, nil
}

/*
 * 在线模型请求串格式
 * http://127.0.0.1:8080/online?biz=[model name]&src=[redis&stream]&dst=[redis&local&json]
                &epoch=[2]&threads=[threads number]&train=[redis key/instance strings]
				&debug=[off]&thd=[threshold]
   src:训练数据为redis还是即时stream (初始化模型如果redis不存在则读取local,模型key为biz值)
   dst:模型存储到redis、local和json
   train:训练数据来源于redis或stream
*/
func (lan *Lands) onlineServeHttp(w http.ResponseWriter, par *util.ModelParam) error {
	lan.log4goline.Info("[Lands-onlineServeHttp] Begin online learning...")
	conn := lan.pool.Get()
	defer conn.Close()
	var base_path_on string

	base_path_on = lan.conf.DataPathBase + par.Biz + "/on/"

	timestamp := time.Now().Format(TimeFormatString)
	err := util.Mkdir(base_path_on + "/" + timestamp)
	if err != nil {
		lan.log4goline.Error(fmt.Sprintf(fmt.Sprintf(JsonError, "[Lands-onlineServeHttp] Online make local directory error."+err.Error())))
		return errors.New("[Lands-onlineServeHttp] Online make local directory error." + err.Error())
	}

	base_path_ws := base_path_on + "workspace/"

	err = util.Mkdir(base_path_ws)
	if err != nil {
		lan.log4goline.Error(fmt.Sprintf(fmt.Sprintf(JsonError, "[Lands-onlineServeHttp] Online make local workspace directory error."+err.Error())))
		return errors.New("[Lands-onlineServeHttp] Online make local workspace directory error." + err.Error())
	}

	//读取离线模型
	//优先读取redis
	lan.log4goline.Info("[Lands-onlineServeHttp] Read online model.")
	var encodemodel string
	key := ModelPrefix + par.Biz
	reply, err := redis.Values(conn.Do("MGET", key))
	if err != nil {
		//redis读取失败则读本地模型
		fs, err := os.Open(base_path_ws + "model.dat")
		if err != nil {
			lan.log4goline.Error(fmt.Sprintf(fmt.Sprintf(JsonError, "[Lands-onlineServeHttp] Offline model must be trained."+err.Error())))
			return errors.New("[Lands-onlineServeHttp] Offline model must be trained." + err.Error())
		}

		defer fs.Close()

		buf := bufio.NewReader(fs)

		line, err := buf.ReadString('\n')
		encodemodel = s.TrimSpace(line)
		if err != nil && err != io.EOF {
			lan.log4goline.Error(fmt.Sprintf(fmt.Sprintf(JsonError, "[Lands-onlineServeHttp] Open offline model error."+err.Error())))
			return errors.New("[Lands-onlineServeHttp] Open offline model error." + err.Error())
		}
	} else {
		if _, err := redis.Scan(reply, &encodemodel); err != nil {
			lan.log4goline.Error(fmt.Sprintln("[Lands-onlineServeHttp] Open offline model from redis error." + err.Error()))
			return errors.New("[Lands-onlineServeHttp] Open offline model from redis error." + err.Error())
		}
	}

	//模型训练
	lan.log4goline.Info("[Lands-onlineServeHttp] Online model training with lock free ftrl.")
	var lff trainer.LockFreeFtrlTrainer
	lff.SetJobName(par.Biz + " online " + timestamp)
	if !lff.Initialize(par.Epoch, par.Threads, false) {
		lan.log4goline.Error("[Lands-onlineServeHttp] Initialize offline model error.")
		return errors.New("[Lands-onlineServeHttp] Initialize offline model error.")
	}
	instances := strings.Split(par.Train, lan.conf.LineSpliter)
	if len(instances) == 0 {
		lan.log4goline.Error("[Lands-onlineServeHttp] Instances number error.")
		return errors.New("[Lands-onlineServeHttp] Instances number error.")
	}
	model, err := lff.TrainOnline(encodemodel, instances)
	if err != nil {
		lan.log4goline.Error("[Lands-onlineServeHttp] Online model training error." + err.Error())
		return errors.New("[Lands-onlineServeHttp] Online model training error." + err.Error())
	}

	//模型写入本地
	lan.log4goline.Info("[Lands-onlineServeHttp] Write model to local directory.")
	util.CopyFile(base_path_ws+"model_bak.dat", base_path_ws+"model.dat")
	fout, err := os.OpenFile(base_path_on+"/"+timestamp+"/model.dat", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer fout.Close()
	if err != nil {
		lan.log4goline.Error("[Lands-onlineServeHttp] Online model write local error." + err.Error())
		return errors.New("[Lands-onlineServeHttp] Online model write local error." + err.Error())
	}

	fout.WriteString(model)
	err = util.CopyFile(base_path_ws+"model.dat", base_path_on+"/"+timestamp+"/model.dat")
	if err != nil {
		lan.log4goline.Error("[Lands-onlineServeHttp] Copy model error." + err.Error())
		return errors.New("[Lands-onlineServeHttp] Copy model error." + err.Error())
	}

	//清理目录
	lan.log4goline.Info("[Lands-onlineServeHttp] Clear local directory.")
	err = util.KeepLatestN(base_path_on, 5)
	if err != nil {
		lan.log4goline.Warn("[Lands-onlineServeHttp] Clear local file error." + err.Error())
		errors.New("[Lands-onlineServeHttp] Clear local file error." + err.Error())
	}

	//接口输出
	fmt.Fprintf(w, model)

	//模型存入redis
	lan.log4goline.Info("[Lands-onlineServeHttp] Write model to redis.")
	if par.Dst == "redis" {
		key := ModelPrefix + par.Biz
		conn.Send("SET", key, model)
		conn.Flush()
		_, err := conn.Receive()
		if err != nil {
			lan.log4goline.Warn(fmt.Sprintln("[Lands-onlineServeHttp] Save model to redis error." + err.Error()))
			return errors.New("[Lands-onlineServeHttp] Save model to redis error." + err.Error())
		}
	}

	lan.log4goline.Info("[Lands-onlineServeHttp] End online learning.")

	return nil
}

/*
 * 离线模型请求串格式
 * http://127.0.0.1:8080/offline?biz=[model name]&src=[hdfs/local]&dst=[redis&local&json]
                &alpha=[0.1]&beta=[0.1]&l1=[10]&l2=[10]&dropout=[0.1]&epoch=[2]
				&push=[push step]&fetch=[fetch step]&threads=[threads number]
				&train=[train file name]&test=[test file name]&debug=[off]&thd=[threshold]
   src:训练、测试数据源为hdfs/local
   dst:模型输出到redis、local和json
   train:训练数据完整路径
   test:测试数据完整路径
*/
func (lan *Lands) offlineServeHttp(w http.ResponseWriter, par *util.ModelParam) error {
	lan.log4goline.Info("[Lands-offlineServeHttp] Begin offline learning...")
	conn := lan.pool.Get()
	defer conn.Close()
	var base_path_off, model_path, train_path, test_path string
	var predict_path string

	//建立模型训练本地路径
	base_path_off = lan.conf.DataPathBase + par.Biz + "/off/"

	timestamp := time.Now().Format(TimeFormatString)
	err := util.Mkdir(base_path_off + "/" + timestamp)
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Offline model make local directory error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Offline model make local directory error." + err.Error())
	}

	base_path_on := lan.conf.DataPathBase + par.Biz + "/on/workspace"

	err = util.Mkdir(base_path_on)
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Make online local workspace directory error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Make online local workspace directory error." + err.Error())
	}

	//挂载数据
	lan.log4goline.Info("[Lands-offlineServeHttp] Mount training data.")
	if par.Src == "hdfs" {
		client, err := lan.createHdfsClient()
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Create hdfs client error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Create hdfs client error." + err.Error())
		}

		model_path = base_path_off + "/" + timestamp + "/model.dat"
		train_path = base_path_off + "/" + timestamp + "/train.dat"
		test_path = base_path_off + "/" + timestamp + "/test.dat"
		predict_path = base_path_off + "/" + timestamp + "/predict.dat"

		client.GetMerge(par.Train, train_path, false)
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Getmerge train data from hdfs to local error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Getmerge train data from hdfs to local error." + err.Error())
		}

		client.GetMerge(par.Test, test_path, false)
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Getmerge test data from hdfs to local error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Getmerge test data from hdfs to local error." + err.Error())
		}

	} else if par.Src == "local" {
		model_path = base_path_off + "/" + timestamp + "/model.dat"
		train_path = base_path_off + "/" + timestamp + "/train.dat"
		test_path = base_path_off + "/" + timestamp + "/test.dat"
		predict_path = base_path_off + "/" + timestamp + "/predict.dat"
		err := util.CopyFile(train_path, par.Train)
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Copy train data from local to local error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Copy train data from local to local error." + err.Error())
		}
		err = util.CopyFile(test_path, par.Test)
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Copy test data from local to local error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Copy test data from local to local error." + err.Error())
		} else {
			lan.log4goline.Error("[Lands-offlineServeHttp] Training data source path error.")
			return errors.New("[Lands-offlineServeHttp] Training data source path error.")
		}
	}

	lan.log4goline.Info(fmt.Sprintf("[Lands-offlineServeHttp] Model path=%s,train path=%s, test path=%s\n",
		model_path,
		train_path,
		test_path))

	//训练数据格式检查及转换
	lan.log4goline.Info("[Lands-offlineServeHttp] Check training data.")
	_, err = lan.checkData(train_path)
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Check train data from local to local error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Check train data from local to local error." + err.Error())
	}
	lan.log4goline.Info("[Lands-offlineServeHttp] Check testing data.")
	_, err = lan.checkData(test_path)
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Check test data from local to local error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Check test data from local to local error." + err.Error())
	}

	//数据抽样
	if !util.UtilFloat64Equal(par.Sample, 1.0) && !util.UtilFloat64Equal(par.Sample, -1.0) {
		lan.log4goline.Info("[Lands-offlineServeHttp] Training data sampling.")
		err = util.FileSampleWithRatio(train_path, par.Sample)
		if err != nil {
			lan.log4goline.Error("[Lands-offlineServeHttp] Train data sampling error." + err.Error())
			return errors.New("[Lands-offlineServeHttp] Train data sampling error." + err.Error())
		}
	}

	//模型训练
	lan.log4goline.Info("[Lands-offlineServeHttp] Offline model training.")
	var fft trainer.FastFtrlTrainer
	fft.SetJobName(par.Biz + " offline " + timestamp)
	if !fft.Initialize(par.Epoch, par.Threads, true, 0, par.Push, par.Fetch) {
		lan.log4goline.Error("[Lands-offlineServeHttp] Initialize ftrl trainer error")
		return errors.New("[Lands-offlineServeHttp] Initialize ftrl trainer error.")
	}

	err = fft.Train(par.Alpha, par.Beta, par.L1, par.L2, par.Dropout, model_path,
		train_path, test_path)
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Training model error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Training model error." + err.Error())
	}

	lan.log4goline.Info("[Lands-offlineServeHttp] Predict testing data.")
	_, err = predictor.Run(3, []string{par.Biz + " offline " + timestamp, test_path,
		model_path,
		predict_path,
		par.Threshold})

	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Predicting model error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Predicting model error." + err.Error())
	}

	err = util.CopyFile(base_path_on+"/model.dat", base_path_off+"/"+timestamp+"/model.dat")
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Copy model error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Copy model error." + err.Error())
	}

	//清理目录
	lan.log4goline.Info("[Lands-offlineServeHttp] Clear local directory." + base_path_off)
	err = util.KeepLatestN(base_path_off, 5)
	if err != nil {
		lan.log4goline.Warn("[Lands-offlineServeHttp] Clear local file error." + err.Error())
		errors.New("[Lands-offlineServeHttp] Clear local file error." + err.Error())
	}

	m, err := fft.ParamServer.SaveEncodeModel()
	if err != nil {
		lan.log4goline.Error("[Lands-offlineServeHttp] Save model error." + err.Error())
		return errors.New("[Lands-offlineServeHttp] Save model error." + err.Error())
	}

	fmt.Fprintf(w, "%s", m)

	//模型存入redis
	lan.log4goline.Info("[Lands-offlineServeHttp] Write model to redis.")
	if par.Dst == "redis" {
		key := ModelPrefix + par.Biz
		conn.Send("SET", key, m)
		conn.Flush()
		_, err := conn.Receive()
		if err != nil {
			lan.log4goline.Error(fmt.Sprintln("[Lands-offlineServeHttp] Save model to redis error." + err.Error()))
			return errors.New("[Lands-offlineServeHttp] Save model to redis error." + err.Error())
		}
	}

	lan.log4goline.Info("[Lands-offlineServeHttp] End offline learning.")
	return nil
}

/*
 * 离线模型请求串格式
 * http://127.0.0.1:8080/predict?biz=[model name]&src=[hdfs/redis/stream]&dst=[local&json]
				&pred=[hdfs file/redis key/instance strings]&debug=[off]&thd=[threshold]
   src:待预测数据源为hdfs/local
   dst:待预测数据输出到local和json
   pred:待预测数据完整路径
*/
func (lan *Lands) predictServeHttp(w http.ResponseWriter, par *util.ModelParam) error {
	lan.log4goline.Info("[Lands-predictServeHttp] Begin predicting...")
	conn := lan.pool.Get()
	defer conn.Close()
	var predict_path string

	base_path_prd := lan.conf.DataPathBase + par.Biz + "/prd/"

	timestamp := time.Now().Format(TimeFormatString)
	base_path_prdtime := base_path_prd + "/" + timestamp

	err := util.Mkdir(base_path_prdtime)
	if err != nil {
		fmt.Fprintf(w, fmt.Sprintf(JsonError, "[Lands-predictServeHttp] make local directory error."+err.Error()))
		return errors.New("[Lands-predictServeHttp] make local directory error." + err.Error())
	}

	base_path_ws := lan.conf.DataPathBase + par.Biz + "/on/workspace/"

	//清理目录
	lan.log4goline.Info("[Lands-predictServeHttp] Clear local directory.")
	err = util.KeepLatestN(base_path_prd, 5)
	if err != nil {
		lan.log4goline.Warn("[Lands-onlineServeHttp] Clear local file error." + err.Error())
		errors.New("[Lands-onlineServeHttp] Clear local file error." + err.Error())
	}

	//模型本地备份
	err = util.CopyFile(base_path_ws+"model_bak.dat", base_path_ws+"model.dat")
	if err != nil {
		lan.log4goline.Error("[Lands-predictServeHttp] Online model write local error." + err.Error())
		return errors.New("[Lands-predictServeHttp] Online model write local error." + err.Error())
	}

	//读取离线模型
	//优先读取redis模型
	lan.log4goline.Info("[Lands-predictServeHttp] Read model from redis.")
	key := ModelPrefix + par.Biz
	var encodemodel string
	reply, err := redis.Values(conn.Do("MGET", key))
	if err != nil {
		lan.log4goline.Error(fmt.Sprintf("[Lands-predictServeHttp] Get model from redis error." + err.Error()))
	} else {
		if _, err := redis.Scan(reply, &encodemodel); err != nil {
			lan.log4goline.Error(fmt.Sprintf("[Lands-onlineServeHttp] Open offline model from redis error." + err.Error()))
		}
	}

	//从redis取到了模型
	if len(encodemodel) != 0 {
		fout, err := os.OpenFile(base_path_ws+"model.dat", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
		defer fout.Close()
		if err != nil {
			lan.log4goline.Error("[Lands-predictServeHttp] Online model from redis write to local error." + err.Error())
		}

		fout.WriteString(encodemodel)
	}

	//挂载数据
	lan.log4goline.Info("[Lands-predictServeHttp] Mount predicting data from local/hdfs/redis.")
	if par.Src == "local" {
		predict_path = base_path_prdtime + "/predict.dat"
		err := util.CopyFile(predict_path, par.Predict)
		if err != nil {
			lan.log4goline.Error("[Lands-predictServeHttp] copy train data from local to local error." + err.Error())
			return errors.New("[Lands-predictServeHttp] copy train data from local to local error." + err.Error())
		}
	} else if par.Src == "hdfs" {
		client, err := lan.createHdfsClient()
		if err != nil {
			lan.log4goline.Error("[Lands-predictServeHttp] Create hdfs client error." + err.Error())
			return errors.New("[Lands-predictServeHttp] Create hdfs client error." + err.Error())
		}

		predict_path = base_path_prdtime + "/predict.dat"

		client.GetMerge(par.Predict, predict_path, false)
		if err != nil {
			lan.log4goline.Error("[Lands-predictServeHttp] Getmerge train data from hdfs to local error." + err.Error())
			return errors.New("[Lands-predictServeHttp] Getmerge train data from hdfs to local error." + err.Error())
		}

	} else if par.Src == "redis" {
		predict_path = base_path_prdtime + "/predict.dat"
		//redis get with key
	} else {
		// run stream
		instances := strings.Split(strings.TrimSpace(par.Predict), lan.conf.LineSpliter)
		if len(instances) == 0 {
			lan.log4goline.Error("[Lands-predictServeHttp] Streaming instances number error.")
			return errors.New("[Lands-predictServeHttp] Streaming instances number error.")
		}

		json, err := predictor.StreamRun(base_path_ws+"model.dat", instances)
		if err != nil {
			lan.log4goline.Error("[Lands-predictServeHttp] Streaming predicting running error." + err.Error())
			return errors.New("[Lands-predictServeHttp] Streaming predicting running error." + err.Error())
		}

		fmt.Fprintf(w, json)
		lan.log4goline.Info("[Lands-predictServeHttp] End predicting.")
		return nil
	}

	json, err := predictor.Run(3, []string{par.Biz + " predict " + timestamp, predict_path,
		base_path_ws + "model.dat",
		base_path_prdtime + "/predict_result.dat",
		par.Threshold})
	if err != nil {
		lan.log4goline.Error("[Lands-predictServeHttp] Predicting running error." + err.Error())
		return errors.New("[Lands-predictServeHttp] Predicting running error." + err.Error())
	}

	fmt.Fprintf(w, json)
	lan.log4goline.Info("[Lands-predictServeHttp] End predicting.")

	return nil
}

func (lan *Lands) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	par := util.ParamParse(r)
	lan.log4goline.Info("[ServeHTTP] Parameters:" + par.String())
	if h, ok := lan.mux[par.Module]; ok {
		err := h(w, par)
		if err != nil {
			lan.log4goline.Error(err)
		}
	}
}
