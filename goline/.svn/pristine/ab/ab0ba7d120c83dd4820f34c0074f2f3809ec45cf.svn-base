package trainer

import (
	"errors"
	"goline/util"
	s "strings"
	"sync"
)

type StreamParser struct {
	Buf  []string
	Idx  int
	Lock sync.Mutex
}

func (sp *StreamParser) Open(instances []string) error {
	if len(instances) == 0 {
		return errors.New("[Open] Instances are empty.")
	}

	sp.Buf = instances
	sp.Idx = 0

	return nil
}

func (sp *StreamParser) Close() error {
	if sp.Idx != len(sp.Buf) {
		return errors.New("[Close] Close stream error.")
	}

	return nil
}

func (sp *StreamParser) ReadSampleMultiThread() (error, float64, util.Pvector) {
	sp.Lock.Lock()
	if sp.Idx >= len(sp.Buf) {
		sp.Lock.Unlock()
		return errors.New("[StreamParser-ReadSampleMultiThread] input value error."), 0., nil
	}

	instance := s.TrimSpace(sp.Buf[sp.Idx])
	sp.Idx++
	sp.Lock.Unlock()
	if len(instance) == 0 {
		return errors.New("[StreamParser-ReadSampleMultiThread] input value length error."), 0., nil
	}

	return util.ParseSample(instance)
}

func (sp *StreamParser) ReadLine() error {
	if sp.Idx != len(sp.Buf)-1 {
		return errors.New("[ReadLine] Close stream error.")
	}

	return nil
}
