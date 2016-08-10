package util

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	eps = 0.00001
)

type ModelParam struct {
	Module, Biz, Src, Dst, Train, Test, Predict, Debug, Threshold string
	Alpha, Beta, L1, L2, Dropout, Sample                          float64
	Push, Fetch, Epoch, Threads                                   int
}

func (mp *ModelParam) String() string {
	return fmt.Sprintf("Module=%s, Biz=%s, Src=%s, Dst=%s, Train=%s, Test=%s, Predict=%s, Debug=%s, Threshold=%s,Alpha=%f, Beta=%f, L1=%f, L2=%f, Dropout=%f, Sample=%f, Push=%d, Fetch=%d, Epoch=%d, Threads=%d",
		mp.Module, mp.Biz, mp.Src, mp.Dst, mp.Train, mp.Test, mp.Predict, mp.Debug, mp.Threshold,
		mp.Alpha, mp.Beta, mp.L1, mp.L2, mp.Dropout, mp.Sample, mp.Push, mp.Fetch, mp.Epoch, mp.Threads)
}

func String2Float64(elem string) float64 {
	i, err := strconv.ParseFloat(elem, 64)
	if err != nil {
		return 0
	}

	return i
}

func String2Int(elem string) int {
	i, err := strconv.ParseInt(elem, 10, 0)
	if err != nil {
		return 0
	}

	return int(i)
}

func ParamParse(r *http.Request) *ModelParam {
	r.ParseForm()
	mp := ModelParam{"offline", "", "hdfs", "json", "", "", "", "off", "0.06", 0.1, 1, 10, 10, 0.1, 1, 10, 10, 2, 8}

	if len(strings.Split(r.URL.String(), "?")) != 0 {
		mp.Module = strings.Split(r.URL.String(), "?")[0]
	}

	if len(r.Form["biz"]) != 0 {
		mp.Biz = r.Form["biz"][0]
	}

	if len(r.Form["src"]) != 0 {
		mp.Src = r.Form["src"][0]
	}

	if len(r.Form["dst"]) != 0 {
		mp.Dst = r.Form["dst"][0]
	}

	if len(r.Form["train"]) != 0 {
		mp.Train = r.Form["train"][0]
	}

	if len(r.Form["test"]) != 0 {
		mp.Test = r.Form["test"][0]
	}

	if len(r.Form["predict"]) != 0 {
		mp.Predict = r.Form["predict"][0]
	}

	if len(r.Form["debug"]) != 0 {
		mp.Debug = r.Form["debug"][0]
	}

	if len(r.Form["thd"]) != 0 {
		mp.Threshold = r.Form["thd"][0]
	}

	if len(r.Form["alpha"]) != 0 && String2Float64(r.Form["alpha"][0]) >= eps {
		mp.Alpha = String2Float64(r.Form["alpha"][0])
	}

	if len(r.Form["beta"]) != 0 && String2Float64(r.Form["beta"][0]) >= eps {
		mp.Beta = String2Float64(r.Form["beta"][0])
	}

	if len(r.Form["l1"]) != 0 && String2Float64(r.Form["l1"][0]) >= eps {
		mp.L1 = String2Float64(r.Form["l1"][0])
	}

	if len(r.Form["l2"]) != 0 && String2Float64(r.Form["l2"][0]) >= eps {
		mp.L2 = String2Float64(r.Form["l2"][0])
	}

	if len(r.Form["dropout"]) != 0 && String2Float64(r.Form["dropout"][0]) >= eps {
		mp.Dropout = String2Float64(r.Form["dropout"][0])
	}

	if len(r.Form["sample"]) != 0 {
		mp.Sample = String2Float64(r.Form["sample"][0])
	}

	if len(r.Form["push"]) != 0 && String2Int(r.Form["push"][0]) != 0 {
		mp.Push = String2Int(r.Form["push"][0])
	}

	if len(r.Form["fetch"]) != 0 && String2Int(r.Form["fetch"][0]) != 0 {
		mp.Fetch = String2Int(r.Form["fetch"][0])
	}

	if len(r.Form["epoch"]) != 0 && String2Int(r.Form["epoch"][0]) != 0 {
		mp.Epoch = String2Int(r.Form["epoch"][0])
	}

	if len(r.Form["threads"]) != 0 && String2Int(r.Form["threads"][0]) != 0 {
		mp.Threads = String2Int(r.Form["threads"][0])
	}

	return &mp
}
