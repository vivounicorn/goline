package trainer

import (
	"bufio"
	"errors"
	"fmt"
	"goline/util"
	"io"
	"os"
	"strconv"
	s "strings"
	"sync"
)

type ParallelFileParser struct {
	Fs    []*os.File
	Bufio []*bufio.Reader
	Lock  sync.Mutex
}

func (fp *ParallelFileParser) OpenFile(filename string, threadnum int) error {
	err := util.SplitFile(filename, threadnum)
	if err != nil {
		return errors.New(fmt.Sprintf("[ParallelFileParser-OpenFile] Split file failed.%s", err.Error()))
	}

	fp.Fs = make([]*os.File, threadnum)
	fp.Bufio = make([]*bufio.Reader, threadnum)
	for i := 0; i < threadnum; i++ {
		fs, err := os.Open(s.TrimSuffix(filename, ".dat") + strconv.Itoa(i) + ".dat")
		if err != nil {
			return errors.New(fmt.Sprintf("[ParallelFileParser-OpenFile] Open file failed.%s", err.Error()))
		}

		fp.Bufio[i] = bufio.NewReader(fs)
		fp.Fs[i] = fs
	}
	return nil
}

func (fp *ParallelFileParser) CloseFile(threadnum int) bool {
	for i := 0; i < threadnum; i++ {
		if len(fp.Fs) > 0 && fp.Fs[i] != nil {
			defer fp.Fs[i].Close()
		}
	}

	return true
}

func (fp *ParallelFileParser) ReadLineImpl(i int) (string, error) {
	if fp.Bufio[i] == nil {
		return "", errors.New("bufio initialize error.")
	}
	line, err := fp.Bufio[i].ReadString('\n')
	line = s.TrimSpace(line)
	if err != nil {
		if err != io.EOF {
			return "", err
		}
	}

	return line, nil
}

func (fp *ParallelFileParser) ReadLine(i int) string {
	fp.Lock.Lock()
	buf, err := fp.ReadLineImpl(i)
	fp.Lock.Unlock()
	if err != nil {
		return ""
	}

	return buf
}

func (fp *ParallelFileParser) ReadSample(i int) (error, float64, util.Pvector) {
	fp.Lock.Lock()
	buf, err := fp.ReadLineImpl(i)
	fp.Lock.Unlock()
	if err != nil || len(buf) == 0 {
		return errors.New("[ParallelFileParser-ReadSample] input value error."), 0., nil
	}

	return util.ParseSample(buf)
}

func (fp *ParallelFileParser) ReadSampleMultiThread(i int) (error, float64, util.Pvector) {
	buf := fp.ReadLine(i)
	if len(buf) == 0 {
		return errors.New("[ParallelFileParser-ReadSampleMultiThread] input value error."), 0., nil
	}

	return util.ParseSample(buf)
}
