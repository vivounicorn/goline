package util

import (
	"time"
)

type StopWatch struct{
	start time.Time
	stop time.Time
}

func (sw *StopWatch) ToSeconds() float64 {	
	return sw.stop.Sub(sw.start).Seconds()
}

func (sw *StopWatch) ToNanoSeconds() int64 {
	return sw.stop.Sub(sw.start).Nanoseconds()
}

func (sw *StopWatch) StartTimer() {
	sw.start = time.Now()
}

func (sw *StopWatch) StopTimer() float64 {
	sw.stop = time.Now()
	return sw.ToSeconds()
}

func (sw *StopWatch) ElapsedTime() float64 {
	return sw.ToSeconds()
}

func (sw *StopWatch) ElapsedTimeNS() int64{
	return sw.ToNanoSeconds()
}