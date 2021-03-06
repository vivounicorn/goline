package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	s "strings"
	"sync"
	"time"
)

const (
	MaxExpNum    = 50
	MinSigmoid   = 10e-15
	MaxSigmoid   = 1. - 10e-15
	FloatEpsilon = 1.192093e-007
)

type Pair struct {
	Index int     `json:"Index"`
	Value float64 `json:"Value"`
}

type Pvector []Pair

type DPair struct {
	First  float64
	Second float64
}

type Dvector []DPair

func (dv Dvector) Less(i, j int) bool {
	if dv[i].First < dv[j].First {
		return true
	}

	return false
}

func (dv Dvector) Len() int {
	return len(dv)
}

func (dv Dvector) Swap(i, j int) {
	var temp DPair = dv[i]
	dv[i] = dv[j]
	dv[j] = temp
}

var ch chan int

//多核并行
func UtilParallelRun(f func(int, *sync.WaitGroup), num_threads int) {
	wg := new(sync.WaitGroup)
	if num_threads == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
		num_threads = runtime.NumCPU()
	}

	wg.Add(num_threads)
	for i := 0; i < num_threads; i++ {
		go f(i, wg)
	}
	wg.Wait()
}

func UtilFloat64Equal(v1 float64, v2 float64) bool {
	return math.Abs(v1-v2) < FloatEpsilon
}

func UtilGreater(v1 float64, v2 float64) bool {
	if UtilFloat64Equal(v1, v2) {
		return false
	}

	return v1 > v2
}

func UtilFloat64Cmp(v1 float64, v2 float64) int {
	if UtilFloat64Equal(v1, v2) {
		return 0
	} else if v1 > v2 {
		return 1
	} else {
		return -1
	}
}

func UtilFloat64GreaterEqual(v1 float64, v2 float64) bool {
	if UtilFloat64Equal(v1, v2) {
		return true
	}

	return v1 > v2
}

func UtilFloat64Less(v1 float64, v2 float64) bool {
	if UtilFloat64Equal(v1, v2) {
		return false
	}

	return v1 < v2
}

func UtilFloat64LessEqual(v1 float64, v2 float64) bool {
	if UtilFloat64Equal(v1, v2) {
		return true
	}

	return v1 < v2
}

func SafeExp(x float64) float64 {
	max_exp := float64(MaxExpNum)
	return math.Exp(math.Max(math.Min(x, max_exp), -max_exp))
}

func Sigmoid(x float64) float64 {
	one := float64(1.0)
	return one / (one + SafeExp(-x))
}

func UniformDistribution() float64 {
	rand.Seed(time.Now().Unix())
	return rand.Float64()
}

func VectorMultiplies(candidate []float64, mul float64) {
	for i := 0; i < len(candidate); i++ {
		candidate[i] = candidate[i] * mul
	}
}

func MaxInt(first int, args ...int) int {
	for _, v := range args {
		if first < v {
			first = v
		}
	}
	return first
}

func MinInt(first int, args ...int) int {
	for _, v := range args {
		if first > v {
			first = v
		}
	}
	return first
}

//判断文件是否存在
func FileExists(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

//样本解析
func ParseSample(buf string) (error, float64, Pvector) {
	if len(buf) == 0 {
		return errors.New("[ParseSample] input value error."), 0., nil
	}

	//要求样本格式为libsvm格式，即：label dim1:val1 dim2:val2 dim3:val3
	var res []string = s.Split(s.TrimSpace(buf), " ")
	var start int
	var length int
	if len(res) < 2 {
		return errors.New("[ParseSample] sample format error." + buf), 0, nil
	}

	if s.Contains(buf, "|f") {
		start = 3
		length = len(res) - 3
	} else {
		start = 1
		length = len(res)
	}

	y, err := strconv.ParseFloat(res[0], 64)
	if err != nil {
		return errors.New("[ParseSample] parse sample error." + err.Error()), 0., nil
	}

	if y < 0. {
		y = 0.
	}

	var x Pvector
	//偏置
	x = append(x, Pair{0, 1.})

	for i := start; i < length; i++ {
		var sp []string = s.Split(res[i], ":")
		if len(sp) != 2 {
			log.Warn("sample format error [idx:val]." + res[i])
			continue
		}

		ix, err := strconv.Atoi(sp[0])
		if err != nil {
			log.Warn("parse sample index error:", err)
			continue
		}
		vl, err := strconv.ParseFloat(sp[1], 64)
		if err != nil {
			log.Warn("parse sample value error:", err)
			continue
		}

		var instance Pair = Pair{ix, vl}
		x = append(x, instance)
	}

	return nil, y, x
}

//拷贝文件
func CopyFile(dstName, srcName string) error {
	src, err := os.Open(srcName)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstName, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)

	if err != nil {
		return err
	}

	return nil
}

//创建文件目录
func Mkdir(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

type dirInfo struct {
	name   string
	mdtime time.Time
}

type dirvector []dirInfo

func (dv dirvector) Less(i, j int) bool {
	if dv[i].mdtime.After(dv[j].mdtime) {
		return true
	}

	return false
}

func (dv dirvector) Len() int {
	return len(dv)
}

func (dv dirvector) Swap(i, j int) {
	var temp dirInfo = dv[i]
	dv[i] = dv[j]
	dv[j] = temp
}

//保留最近topN文件
func KeepLatestN(path string, topn int) error {
	if topn <= 0 {
		return errors.New("[KeepLatestN] topn parameter value error.")
	}

	var dirs dirvector
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, fi := range dir {
		if !fi.IsDir() {
			continue
		}
		dirs = append(dirs, dirInfo{fi.Name(), fi.ModTime()})
	}

	sort.Sort(dirs)
	for i := topn; i < len(dirs); i++ {
		if dirs[i].name == "workspace" {
			continue
		}

		err := os.RemoveAll(path + "/" + dirs[i].name)
		if err != nil {
			return err
		}
	}

	return nil
}

func round2(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

//四舍五入
func Round(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round2(num*output)) / output
}

//获取文件大小
func FileSize(filename string) (int64, error) {
	if !FileExists(filename) {
		return 0, errors.New(fmt.Sprintf("[FileSize] source file %s not exists.", filename))
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, err

	}

	fileSize := fileInfo.Size()
	return fileSize, nil
}

//文件抽样
func FileSample(src string, size int64) error {
	if !FileExists(src) {
		return errors.New(fmt.Sprintf("[FileSample] source file %s not exists.", src))
	}

	orgsize, err := FileSize(src)
	if err != nil {
		return err
	}

	if orgsize <= size || size <= 0 {
		return nil
	}

	err = CopyFile(src+".bak", src)
	if err != nil {
		return err
	}

	ratio := float64(size) / float64(orgsize)

	fs, err := os.Open(src + ".bak")
	if err != nil {
		return errors.New("[FileSample] Open file failed." + err.Error())
	}

	defer fs.Close()

	buf := bufio.NewReader(fs)

	fout, err := os.OpenFile(src, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		return errors.New("[FileSample] sample data writing error." + err.Error())
	}

	randoms := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				continue
			}
		}
		line = s.TrimSpace(line)
		val := randoms.Float64()
		if val >= 0 && val < ratio {
			fout.WriteString(line + "\n")
		}
	}

	defer fout.Close()
	return nil
}

//文件抽样
func FileSampleWithRatio(src string, ratio float64) error {
	if !FileExists(src) {
		return errors.New(fmt.Sprintf("[FileSample] source file %s not exists.", src))
	}

	_, err := FileSize(src)
	if err != nil {
		return err
	}

	if ratio <= -1 || ratio > 1 {
		return nil
	}

	err = CopyFile(src+".bak", src)
	if err != nil {
		return err
	}

	fs, err := os.Open(src + ".bak")
	if err != nil {
		return errors.New("[FileSample] Open file failed." + err.Error())
	}

	defer fs.Close()

	buf := bufio.NewReader(fs)

	fout, err := os.OpenFile(src, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		return errors.New("[FileSample] sample data writing error." + err.Error())
	}

	randoms := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				continue
			}
		}
		line = s.TrimSpace(line)
		//样本整体采样
		if ratio > 0 {
			val := randoms.Float64()
			if val >= 0 && val < ratio {
				fout.WriteString(line + "\n")
			}
		} else { //负样本采样，正样本保留
			sp := s.Split(line, " ")
			if len(sp) > 0 {
				y, err := strconv.ParseFloat(sp[0], 64)
				if err != nil {
					return errors.New("[FileSample] sub sample data label error." + err.Error())
				}

				if y > 0. {
					fout.WriteString(line + "\n")
				} else {
					val := randoms.Float64()
					if val >= 0 && val < -ratio {
						fout.WriteString(line + "\n")
					}
				}

			} else {
				return errors.New("[FileSample] sub sample data writing error.")
			}

		}
	}

	defer fout.Close()
	return nil
}

func Write2File(filename string, content string) error {
	if len(filename) == 0 || len(content) == 0 {
		return errors.New("[Tools-WriteToFile] Input filename error or content error.")
	}

	fout, err := os.OpenFile(filename+".assess", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer fout.Close()
	if err != nil {
		return errors.New("[Tools-WriteToFile] Input filename error or content error." + err.Error())
	}

	fout.WriteString(content)

	return nil
}

func SplitFile(filename string, num int) error {
	file, err := os.Open(filename)
	if err != nil {
		return errors.New("[Tools-SplitFile] Open file error." + err.Error())
	}

	defer file.Close()
	finfo, err := file.Stat()
	if err != nil {
		return errors.New("[Tools-SplitFile] Get file info failed:")
	}

	size := (int(finfo.Size()) - 1) / num

	bufsize := 1024 * 1024
	if size < bufsize {
		bufsize = size
	}

	buf := make([]byte, bufsize)

	for i := 0; i < num; i++ {
		copylen := 0
		newfilename := s.TrimSuffix(filename, ".dat") + strconv.Itoa(i) + ".dat"
		newfile, err1 := os.OpenFile(newfilename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
		defer newfile.Close()
		if err != nil {
			return errors.New("[Tools-SplitFile] Create file error." + err1.Error())
		}

		for copylen < size {
			n, err2 := file.Read(buf)
			if err2 != nil && err2 != io.EOF {
				return errors.New("[Tools-SplitFile] Read file error." + err2.Error())
			}

			if n <= 0 {
				break
			}

			w_buf := buf[:n]
			newfile.Write(w_buf)
			copylen += n
		}
	}

	return nil
}

func ReadAll(path string) (string, error) {
	fi, err := os.Open(path)
	if err != nil {
		return "", errors.New("[Tools-ReadAll] Read all file in memory error." + err.Error())
	}
	defer fi.Close()
	fd, err := ioutil.ReadAll(fi)
	if err != nil {
		return "", errors.New("[Tools-ReadAll] Read all file in memory error." + err.Error())
	}

	return string(fd), nil
}
