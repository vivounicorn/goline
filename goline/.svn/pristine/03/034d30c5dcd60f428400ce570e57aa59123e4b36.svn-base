package util

import (
	"goline/deps/log4go"
)

var log log4go.Logger

func init() {
	log = make(log4go.Logger)
}

func InitLogger(conf string) {
	log.LoadConfiguration(conf)
}

func GetLogger() log4go.Logger {
	return log
}
