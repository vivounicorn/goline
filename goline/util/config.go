package util

type RedisConfig struct {
	Host           string `json:"Host"`
	ConnectTimeout int64  `json:"ConnectTimeout"`
	ReadTimeout    int64  `json:"ReadTimeout"`
	WriteTimeout   int64  `json:"WriteTimeout"`
	MaxIdle        int    `json:"MaxIdle"`
	IdleTimeout    int64  `json:"IdleTimeout"`
}

type Config struct {
	DataPathBase  string      `json:"DataPathBase"`
	LineSpliter   string      `json:"LineSpliter"`   //online learning instance spliter.
	SampleSpliter string      `json:"SampleSpliter"` //instance spliter between features.
	HadoopUser    string      `json:"HadoopUser"`
	NameNodes     string      `json:"NameNodes"`
	LogModule     string      `json:"LogModule"`
	Redis         RedisConfig `json:"Redis"`
}
