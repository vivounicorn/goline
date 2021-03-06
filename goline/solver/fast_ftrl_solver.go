package solver

import (
	"errors"
	"fmt"
	"goline/deps/log4go"
	"goline/util"
	"math"
	"sync"
)

const (
	ParamGroupSize = 10
	FetchStep      = 3
	PushStep       = 3
)

func calc_group_num(n int) int {
	return (n + ParamGroupSize - 1) / ParamGroupSize
}

type FtrlParamServer struct {
	FtrlSolver

	ParamGroupNum int
	LockSlots     []sync.Mutex
	log           log4go.Logger
}

type FtrlWorker struct {
	FtrlSolver

	ParamGroupNum  int
	ParamGroupStep []int
	PushStep       int
	FetchStep      int

	NUpdate []float64
	ZUpdate []float64
	log     log4go.Logger
}

func (fps *FtrlParamServer) Initialize(
	alpha float64,
	beta float64,
	l1 float64,
	l2 float64,
	n int,
	dropout float64) error {

	fps.log = util.GetLogger()
	if !fps.FtrlSolver.Initialize(alpha, beta, l1, l2, n, dropout) {
		fps.log.Error("[FtrlParamServer-Initialize] Fast ftrl solver initialize error.")
		return errors.New("[FtrlParamServer-Initialize] Fast ftrl solver initialize error.")
	}

	fps.ParamGroupNum = calc_group_num(n)
	fps.LockSlots = make([]sync.Mutex, fps.ParamGroupNum)

	fps.Init = true
	return nil
}

func (fps *FtrlParamServer) Construct(path string) error {
	err := fps.FtrlSolver.Construct(path)
	if err != nil {
		fps.log.Error(fmt.Sprintf("[FtrlParamServer-Construct] Restore fast ftrl solver error.", err.Error()))
		return errors.New(fmt.Sprintf("[FtrlParamServer-Construct] Restore fast ftrl solver error.", err.Error()))
	}

	fps.ParamGroupNum = calc_group_num(fps.FtrlSolver.Featnum)
	fps.LockSlots = make([]sync.Mutex, fps.ParamGroupNum)

	fps.Init = true
	return nil
}

func (fps *FtrlParamServer) FetchParamGroup(n []float64, z []float64, group int) error {
	if !fps.FtrlSolver.Init {
		fps.log.Error("[FtrlParamServer-FetchParamGroup] Initialize fast ftrl solver error.")
		return errors.New("[FtrlParamServer-FetchParamGroup] Initialize fast ftrl solver error.")
	}

	var start int = group * ParamGroupSize
	var end int = util.MinInt((group+1)*ParamGroupSize, fps.FtrlSolver.Featnum)

	fps.LockSlots[group].Lock()
	for i := start; i < end; i++ {
		n[i] = fps.FtrlSolver.N[i]
		z[i] = fps.FtrlSolver.Z[i]
	}
	fps.LockSlots[group].Unlock()

	return nil
}

func (fps *FtrlParamServer) FetchParam(n []float64, z []float64) error {
	if !fps.FtrlSolver.Init {
		fps.log.Error("[FtrlParamServer-FetchParam] Initialize fast ftrl solver error.")
		return errors.New("[FtrlParamServer-FetchParam] Initialize fast ftrl solver error.")
	}

	for i := 0; i < fps.ParamGroupNum; i++ {
		err := fps.FetchParamGroup(n, z, i)
		if err != nil {
			fps.log.Error(fmt.Sprintf("[FtrlParamServer-FetchParam] Initialize fast ftrl solver error.", err.Error()))
			return errors.New(fmt.Sprintf("[FtrlParamServer-FetchParam] Initialize fast ftrl solver error.", err.Error()))
		}
	}
	return nil
}

func (fps *FtrlParamServer) PushParamGroup(n []float64, z []float64, group int) error {
	if !fps.FtrlSolver.Init {
		fps.log.Error("[FtrlParamServer-PushParamGroup] Initialize fast ftrl solver error.")
		return errors.New("[FtrlParamServer-PushParamGroup] Initialize fast ftrl solver error.")
	}

	var start int = group * ParamGroupSize
	var end int = util.MinInt((group+1)*ParamGroupSize, fps.FtrlSolver.Featnum)

	fps.LockSlots[group].Lock()
	for i := start; i < end; i++ {
		fps.FtrlSolver.N[i] += n[i]
		fps.FtrlSolver.Z[i] += z[i]
		n[i] = 0
		z[i] = 0
	}
	fps.LockSlots[group].Unlock()
	return nil
}

func (fw *FtrlWorker) Initialize(
	param_server *FtrlParamServer,
	push_step int,
	fetch_step int) bool {

	fw.FtrlSolver.Alpha = param_server.Alpha
	fw.FtrlSolver.Beta = param_server.Beta
	fw.FtrlSolver.L1 = param_server.L1
	fw.FtrlSolver.L2 = param_server.L2
	fw.FtrlSolver.Featnum = param_server.Featnum
	fw.FtrlSolver.Dropout = param_server.Dropout

	fw.NUpdate = make([]float64, fw.FtrlSolver.Featnum)
	fw.ZUpdate = make([]float64, fw.FtrlSolver.Featnum)
	fw.SetFloatZero(fw.NUpdate, fw.FtrlSolver.Featnum)
	fw.SetFloatZero(fw.ZUpdate, fw.FtrlSolver.Featnum)

	fw.N = make([]float64, fw.FtrlSolver.Featnum)
	fw.Z = make([]float64, fw.FtrlSolver.Featnum)
	if param_server.FetchParam(fw.N, fw.Z) != nil {
		return false
	}

	fw.ParamGroupNum = calc_group_num(fw.FtrlSolver.Featnum)
	fw.ParamGroupStep = make([]int, fw.ParamGroupNum)
	for i := 0; i < fw.ParamGroupNum; i++ {
		fw.ParamGroupStep[i] = 0
	}

	fw.PushStep = push_step
	fw.FetchStep = fetch_step

	fw.log = util.GetLogger()

	fw.FtrlSolver.Init = true
	return fw.FtrlSolver.Init
}

func (fw *FtrlWorker) Reset(param_server *FtrlParamServer) error {
	if !fw.FtrlSolver.Init {
		fw.log.Error("[FtrlWorker-Reset] Initialize fast ftrl solver error.")
		return errors.New("[FtrlWorker-Reset] Initialize fast ftrl solver error.")
	}

	err := param_server.FetchParam(fw.FtrlSolver.N, fw.FtrlSolver.Z)
	if err != nil {
		fw.log.Error(fmt.Sprintf("[FtrlWorker-Reset] Initialize fast ftrl solver error.", err.Error()))
		return errors.New(fmt.Sprintf("[FtrlWorker-Reset] Initialize fast ftrl solver error.", err.Error()))
	}

	for i := 0; i < fw.ParamGroupNum; i++ {
		fw.ParamGroupStep[i] = 0
	}
	return nil
}

func (fw *FtrlWorker) Update(
	x util.Pvector,
	y float64,
	param_server *FtrlParamServer) float64 {

	if !fw.FtrlSolver.Init {
		return 0.
	}

	var weights util.Pvector = make(util.Pvector, fw.FtrlSolver.Featnum)
	var gradients []float64 = make([]float64, fw.FtrlSolver.Featnum)
	var wTx float64 = 0.

	for i := 0; i < len(x); i++ {
		item := x[i]
		if util.UtilGreater(fw.FtrlSolver.Dropout, 0.0) {
			rand_prob := util.UniformDistribution()
			if rand_prob < fw.FtrlSolver.Dropout {
				continue
			}
		}
		var idx int = item.Index
		if idx >= fw.FtrlSolver.Featnum {
			continue
		}

		//获取w权重值
		var val float64 = fw.FtrlSolver.GetWeight(idx)
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
		var g int = i / ParamGroupSize

		if fw.ParamGroupStep[g]%fw.FetchStep == 0 {
			param_server.FetchParamGroup(
				fw.FtrlSolver.N,
				fw.FtrlSolver.Z,
				g)
		}

		var w_i float64 = weights[k].Value
		var grad_i float64 = gradients[k]
		var sigma float64 = (math.Sqrt(fw.FtrlSolver.N[i]+grad_i*grad_i) - math.Sqrt(fw.FtrlSolver.N[i])) / fw.FtrlSolver.Alpha

		fw.FtrlSolver.Z[i] += grad_i - sigma*w_i
		fw.FtrlSolver.N[i] += grad_i * grad_i
		fw.ZUpdate[i] += grad_i - sigma*w_i
		fw.NUpdate[i] += grad_i * grad_i

		if fw.ParamGroupStep[g]%fw.PushStep == 0 {
			param_server.PushParamGroup(fw.NUpdate, fw.ZUpdate, g)
		}

		fw.ParamGroupStep[g] += 1
	}

	return pred
}

func (fw *FtrlWorker) PushParam(param_server *FtrlParamServer) error {
	if !fw.FtrlSolver.Init {
		fw.log.Error("[FtrlWorker-PushParam] Initialize fast ftrl solver error.")
		return errors.New("[FtrlWorker-PushParam] Initialize fast ftrl solver error.")
	}

	for i := 0; i < fw.ParamGroupNum; i++ {
		err := param_server.PushParamGroup(fw.NUpdate, fw.ZUpdate, i)
		if err != nil {
			fw.log.Error(fmt.Sprintf("[FtrlWorker-PushParam] Initialize fast ftrl solver error.", err.Error()))
			return errors.New(fmt.Sprintf("[FtrlWorker-PushParam] Initialize fast ftrl solver error.", err.Error()))
		}
	}

	return nil
}
