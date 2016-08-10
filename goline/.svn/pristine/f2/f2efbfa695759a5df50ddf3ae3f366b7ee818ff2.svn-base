package util

import (
	"github.com/garyburd/redigo/redis"
	"time"
)

func InitRedisPool(redisConf *RedisConfig) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     redisConf.MaxIdle,
		IdleTimeout: time.Duration(redisConf.IdleTimeout) * time.Millisecond,
		Dial: func() (redis.Conn, error) {
			c, err := redis.DialTimeout("tcp", redisConf.Host,
				time.Duration(redisConf.ConnectTimeout)*time.Millisecond,
				time.Duration(redisConf.ReadTimeout)*time.Millisecond,
				time.Duration(redisConf.WriteTimeout)*time.Millisecond)
			if err != nil {
				return nil, err
			}

			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
