package solver

import (
	"encoding/json"
	"errors"
	"fmt"
	"goline/deps/log4go"
	"goline/util"
	"io/ioutil"
	"math"
	"os"
	"strconv"
)

const (
	DefaultAlpha = 0.15
	DefaultBeta  = 1.0
	DefaultL1    = 1.0
	DefaultL2    = 1.0
)

type FtrlSolver struct {
	Alpha   float64 `json:"Alpha"`
	Beta    float64 `json:"Beta"`
	L1      float64 `json:"L1"`
	L2      float64 `json:"L2"`
	Featnum int     `json:"Featnum"`
	Dropout float64 `json:"Dropout"`

	N []float64 `json:"N"`
	Z []float64 `json:"Z"`

	Weights util.Pvector `json:"Weights"`

	Init bool `json:"Init"`
}

func (fs *FtrlSolver) SetFloatZero(x []float64, n int) {
	for i := 0; i < n; i++ {
		x[i] = 0
	}
}

func (fs *FtrlSolver) Initialize(
	alpha float64,
	beta float64,
	l1 float64,
	l2 float64,
	n int,
	dropout float64) bool {
	fs.Alpha = alpha
	fs.Beta = beta
	fs.L1 = l1
	fs.L2 = l2
	fs.Featnum = n
	fs.Dropout = dropout

	fs.N = make([]float64, fs.Featnum)
	fs.Z = make([]float64, fs.Featnum)

	fs.SetFloatZero(fs.N, n)
	fs.SetFloatZero(fs.Z, n)
	fs.Init = true
	return fs.Init
}

func (fs *FtrlSolver) Construct(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	var fls FtrlSolver
	b, err2 := ioutil.ReadAll(file)
	if err2 != nil {
		return err2
	}
	err = json.Unmarshal(b, &fls)
	if err != nil {
		return err
	}

	fs.Alpha = fls.Alpha
	fs.Beta = fls.Beta
	fs.Dropout = fls.Dropout
	fs.Featnum = fls.Featnum
	fs.L1 = fls.L1
	fs.N = fls.N
	fs.Z = fls.Z
	fs.Init = fls.Init
	return nil
}

//计算每个维度特征值权重
func (fs *FtrlSolver) GetWeight(idx int) float64 {
	var sign float64 = 1.
	var val float64 = 0.
	if idx >= len(fs.Z) {
		return 0.
	}

	if fs.Z[idx] < 0 {
		sign = -1.
	}

	if util.UtilFloat64Less(sign*fs.Z[idx], fs.L1) {
		val = 0.
	} else {
		val = (sign*fs.L1 - fs.Z[idx]) / ((fs.Beta+math.Sqrt(fs.N[idx]))/fs.Alpha + fs.L2)
	}

	return val
}

//更新权重方法
func (fs *FtrlSolver) Update(x util.Pvector, y float64) float64 {
	if !fs.Init {
		return 0
	}

	var weights util.Pvector = make(util.Pvector, fs.Featnum)
	var gradients []float64 = make([]float64, fs.Featnum)

	var wTx float64 = 0.

	for i := 0; i < len(x); i++ {
		item := x[i]
		if util.UtilGreater(fs.Dropout, 0.0) {
			rand_prob := util.UniformDistribution()
			if rand_prob < fs.Dropout {
				continue
			}
		}

		var idx int = item.Index
		if idx >= fs.Featnum {
			continue
		}

		//获取w权重值
		var val float64 = fs.GetWeight(idx)
		//建立w权重数组
		weights = append(weights, util.Pair{idx, val})
		//每个样本梯度值默认赋值为样本x本身
		gradients = append(gradients, item.Value)
		//计算仿射函数wT*x的值
		wTx += val * item.Value
	}

	//计算模型预估值
	var pred float64 = util.Sigmoid(wTx)
	//计算p_t-y_t值，为计算每个样本的梯度做准备
	var grad float64 = pred - y
	//计算g_i = (p_t-y_t)*x_i
	util.VectorMultiplies(gradients, grad)

	for k := 0; k < len(weights); k++ {
		var i int = weights[k].Index
		var w_i float64 = weights[k].Value
		var grad_i float64 = gradients[k]
		var sigma float64 = (math.Sqrt(fs.N[i]+grad_i*grad_i) - math.Sqrt(fs.N[i])) / fs.Alpha
		//z_i=z_i+g_i-sigma_i*w_(t,i)
		fs.Z[i] += grad_i - sigma*w_i
		//n_i=n_i+g_i*g_i
		fs.N[i] += grad_i * grad_i
	}

	return pred
}

func (fs *FtrlSolver) Predict(x util.Pvector) float64 {
	if !fs.Init {
		return 0
	}

	var wTx float64 = 0.
	for i := 0; i < len(x); i++ {
		idx := x[i].Index
		val := fs.GetWeight(idx)
		wTx += val * x[i].Value
	}

	pred := util.Sigmoid(wTx)
	return pred
}

func (fs *FtrlSolver) ToString(util.Pvector) string {
	if !fs.Init {
		return ""
	}

	var str string = ""
	for i := 0; i < fs.Featnum; i++ {
		val := fs.GetWeight(i)
		if val != 0 {
			str = str + "(" + strconv.Itoa(i) + "," + FloatToString(val) + ") "
			fs.Weights[i] = util.Pair{i, fs.GetWeight(i)}
		}
	}

	return str
}

func (fs *FtrlSolver) SaveModel(path string) error {
	log := util.GetLogger()
	if !fs.Init {
		log.Error("[FtrlSolver-SaveModel] Ftrl solver initialize error.")
		return errors.New("[FtrlSolver-SaveModel] Ftrl solver initialize error.")
	}

	file, err := os.Create(path)
	if err != nil {
		log.Error(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err.Error()))
	}

	fs.Weights = make(util.Pvector, fs.Featnum)
	for i := 0; i < fs.Featnum; i++ {
		val := util.Round(fs.GetWeight(i), 5)
		fs.Weights[i] = util.Pair{i, val}
	}

	b, err2 := json.Marshal(fs)
	if err2 != nil {
		log.Error(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err2.Error()))
		return errors.New(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err2.Error()))
	}

	_, err = file.Write(b)
	if err != nil {
		log.Error(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err2.Error()))
		return errors.New(fmt.Sprintf("[FtrlSolver-SaveModel] Ftrl solver save model error.%s", err2.Error()))
	}

	return nil
}

func (fs *FtrlSolver) SaveEncodeModel() (string, error) {
	log := util.GetLogger()
	if !fs.Init {
		log.Error("[FtrlSolver-SaveEncodeModel] Ftrl solver initialize error.")
		return "", errors.New("[FtrlSolver-SaveEncodeModel] Ftrl solver initialize error.")
	}

	fs.Weights = make(util.Pvector, fs.Featnum)
	for i := 0; i < fs.Featnum; i++ {
		val := util.Round(fs.GetWeight(i), 5)
		fs.Weights[i] = util.Pair{i, val}
	}

	b, err := json.Marshal(fs)
	if err != nil {
		log.Error(fmt.Sprintf("[FtrlSolver-SaveEncodeModel] Ftrl solver save model error.%s", err.Error()))
		return "", errors.New(fmt.Sprintf("[FtrlSolver-SaveEncodeModel] Ftrl solver save model error.%s", err.Error()))
	}

	return string(b), nil
}

type LRModel struct {
	Model map[int]float64
	Init  bool
	log   log4go.Logger
}

func (lr *LRModel) Initialize(path string) error {
	lr.log = util.GetLogger()
	file, err := os.Open(path)
	if err != nil {
		lr.log.Error(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err.Error()))
	}

	defer file.Close()

	var fls FtrlSolver
	m, err := ioutil.ReadAll(file)
	if err != nil {
		lr.log.Error(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err.Error()))
		return errors.New(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err.Error()))
	}

	err2 := json.Unmarshal(m, &fls)
	if err2 != nil {
		lr.log.Error(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err2.Error()))
		return errors.New(fmt.Sprintf("[LRModel-Initialize] Lr model initialize error.%s", err2.Error()))
	}

	lr.Model = make(map[int]float64)
	for i := 0; i < len(fls.Weights); i++ {
		lr.Model[fls.Weights[i].Index] = fls.Weights[i].Value
	}

	lr.Init = true

	return nil
}

func (lr *LRModel) Predict(x util.Pvector) float64 {
	if !lr.Init {
		return 0
	}

	var wTx float64 = 0.
	for i := 0; i < len(x); i++ {
		item := x[i]
		wTx += lr.Model[item.Index] * item.Value
	}

	var pred float64 = util.Sigmoid(wTx)
	return pred
}

func FloatToString(input_num float64) string {
	return strconv.FormatFloat(input_num, 'f', 6, 64)
}

func (lr *LRModel) ToString() string {
	if !lr.Init {
		return ""
	}

	var str string = ""
	for k, v := range lr.Model {
		str = str + "(" + strconv.Itoa(k) + "," + FloatToString(v) + ") "
	}

	return str
}
