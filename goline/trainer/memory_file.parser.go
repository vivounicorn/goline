package trainer

import (
	"errors"
	"goline/util"
	s "strings"
	"sync"
)

type MemoryFileParser struct {
	memory []string
	mindex []int
	lock   []sync.Mutex
	length int
}

func (fp *MemoryFileParser) OpenFile(filename string, threadnum int) error {
	str, err := util.ReadAll(filename)
	if err != nil {
		return err
	}

	fp.memory = s.Split(str, "\n")
	fp.length = len(fp.memory)
	if fp.length == 0 || fp.length < threadnum {
		return errors.New("memory empty.")
	}

	fp.mindex = make([]int, threadnum)
	for i := 0; i < threadnum; i++ {
		fp.mindex[i] = i * fp.length / threadnum
	}
	fp.lock = make([]sync.Mutex, threadnum)

	return nil
}

func (fp *MemoryFileParser) CloseFile(threadnum int) bool {
	for i := 0; i < threadnum; i++ {
		fp.mindex[i] = 0
	}

	return true
}

func (fp *MemoryFileParser) ReadLineImpl(i int) (string, error) {
	if fp.mindex[i] == 0 && i != 0 {
		return "", errors.New("bufio initialize error.")
	}
	if fp.mindex[i] >= fp.length {
		return "", errors.New("index out of range.")
	}
	line := fp.memory[fp.mindex[i]]
	line = s.TrimSpace(line)

	return line, nil
}

func (fp *MemoryFileParser) ReadLine(i int) string {
	fp.lock[i].Lock()
	buf, err := fp.ReadLineImpl(i)
	if err != nil {
		return ""
	}

	fp.mindex[i]++
	fp.lock[i].Unlock()

	return buf
}

func (fp *MemoryFileParser) ReadSample(i int) (error, float64, util.Pvector) {
	fp.lock[i].Lock()
	buf, err := fp.ReadLineImpl(i)

	if err != nil || len(buf) == 0 {
		return errors.New("[MemoryFileParser-ReadSample] input value error."), 0., nil
	}
	fp.mindex[i]++
	fp.lock[i].Unlock()
	return util.ParseSample(buf)
}

func (fp *MemoryFileParser) ReadSampleMultiThread(i int) (error, float64, util.Pvector) {
	//	var timer util.StopWatch
	//	timer.StartTimer()
	buf := fp.ReadLine(i)
	if len(buf) == 0 {
		return errors.New("[MemoryFileParser-ReadSampleMultiThread] input value error."), 0., nil
	}

	//	fmt.Print("time is:")
	//	fmt.Println(timer.StopTimer())

	return util.ParseSample(buf)
}
