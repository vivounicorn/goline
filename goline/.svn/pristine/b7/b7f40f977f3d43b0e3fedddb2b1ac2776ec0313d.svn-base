package trainer

import (
	"bufio"
	"errors"
	"fmt"
	"goline/util"
	"io"
	"os"
	s "strings"
	"sync"
)

type FileParser struct {
	Buf     string
	BufSize int
	Fs      *os.File
	Bufio   *bufio.Reader
	Lock    sync.Mutex
}

func (fp *FileParser) FileExists(filename string) error {
	fs, err := os.Open(filename)
	if err != nil {
		return errors.New(fmt.Sprintf("[FileParser-FileExists] Open file failed.%s", err.Error()))
	}

	defer fs.Close()
	return nil
}

func (fp *FileParser) OpenFile(filename string) error {
	fs, err := os.Open(filename)
	if err != nil {
		return errors.New(fmt.Sprintf("[FileParser-OpenFile] Open file failed.%s", err.Error()))
	}

	fp.Fs = fs
	fp.Bufio = bufio.NewReader(fs)
	return nil
}

func (fp *FileParser) CloseFile() bool {
	if fp.Fs != nil {
		defer fp.Fs.Close()
	}

	return true
}

func (fp *FileParser) ReadLineImpl() string {
	if fp.Bufio == nil {
		return "-1"
	}
	line, err := fp.Bufio.ReadString('\n')
	line = s.TrimSpace(line)
	if err != nil {
		if err == io.EOF {
			return "0"
		} else {
			return "-1"
		}
	}

	return line
}

func (fp *FileParser) ReadLine() string {
	fp.Lock.Lock()
	buf := fp.ReadLineImpl()
	fp.Lock.Unlock()
	if buf != "0" && buf != "-1" {
		return buf
	}

	return ""
}

func (fp *FileParser) ReadSample() (error, float64, util.Pvector) {
	fp.Lock.Lock()
	buf := fp.ReadLineImpl()
	fp.Lock.Unlock()
	if buf == "0" || buf == "-1" {
		return errors.New("[ReadSample] input value error"), 0., nil
	}

	return util.ParseSample(buf)
}

func (fp *FileParser) ReadSampleMultiThread() (error, float64, util.Pvector) {
	buf := fp.ReadLine()
	if len(buf) == 0 {
		return errors.New("[ReadSampleMultiThread] input value error"), 0., nil
	}

	return util.ParseSample(buf)
}
