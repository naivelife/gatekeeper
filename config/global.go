package config

import (
	"net"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/jinzhu/gorm"

	"git.baijiahulian.com/plt/go-common/util/log"
	commonRedis "git.baijiahulian.com/plt/go-common/util/redis"
)

var (
	TimeLocation *time.Location
	LocalIP      net.IP

	AuthConf *AuthConfig
	BaseConf *BaseConfig

	DB    *gorm.DB
	Redis commonRedis.Redis

	SysLog    log.Logger
	AccessLog log.Logger

	Conf *string
)

func RedisPipeline(pip ...func(c redis.Conn)) error {
	c := Redis.GetConnPool().Get()
	defer c.Close()
	for _, f := range pip {
		f(c)
	}
	c.Flush()
	return nil
}

func RedisDo(commandName string, args ...interface{}) (interface{}, error) {
	c := Redis.GetConnPool().Get()
	defer c.Close()
	reply, err := c.Do(commandName, args...)
	if err != nil {
		return nil, err
	}
	return reply, err
}
