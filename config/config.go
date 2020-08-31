package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"git.baijiahulian.com/plt/go-common/model/config"
	"git.baijiahulian.com/plt/go-common/util/log"
	"git.baijiahulian.com/plt/go-common/util/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"

	"gatekeeper/util"
)

// 初始化
func Init(configPath string) error {

	var err error

	// 设置ip信息，优先设置便于日志打印
	ips := util.GetLocalIPs()
	if len(ips) > 0 {
		LocalIP = ips[0]
	}

	// 初始化鉴权信息
	AuthConf = &AuthConfig{}
	if err = parseConfig(configPath+"admin.json", AuthConf); err != nil {
		return err
	}

	// 加载base配置
	BaseConf = &BaseConfig{}
	if err = parseConfig(configPath+"base.json", BaseConf); err != nil {
		return err
	}

	// 加载redis配置并且初始化redis
	redisConf := &RedisConfig{}
	if err = parseConfig(configPath+"redis.json", redisConf); err != nil {
		return err
	}
	Redis, err = redis.NewRedisClusterByConfig(&config.RedisConfig{
		Addr:        []string{fmt.Sprintf("%s:%d", redisConf.Host, redisConf.Port)},
		IdleMax:     redisConf.MaxIdle,
		ActiveMax:   redisConf.MaxActive,
		IdleTimeout: redisConf.IdleTimeout,
		Password:    redisConf.Password,
	}, nil)
	if err != nil {
		return err
	}

	// 加载mysql配置,并且初始化db
	mysqlConf := &MySQLConfig{}
	if err := parseConfig(configPath+"mysql.json", mysqlConf); err != nil {
		return err
	}
	DB, err = gorm.Open("mysql", mysqlConf.DataSourceName)
	if err != nil {
		return err
	}
	if err := DB.DB().Ping(); err != nil {
		return err
	}
	DB.DB().SetMaxOpenConns(mysqlConf.MaxOpenConn)
	DB.DB().SetMaxIdleConns(mysqlConf.MaxIdleConn)
	DB.DB().SetConnMaxLifetime(time.Second * time.Duration(mysqlConf.MaxConnLifeTime))

	// 设置时区
	if location, err := time.LoadLocation(BaseConf.TimeLocation); err != nil {
		return err
	} else {
		TimeLocation = location
	}

	// 设置日志
	SysLog = log.NewDefaultLogrus()
	SysLog.SetOutput(BaseConf.SysLog)
	SysLog.SetLevel(BaseConf.SysLog.LogLevel)

	AccessLog = log.NewDefaultLogrus()
	AccessLog.SetOutput(BaseConf.AccessLog)
	AccessLog.SetLevel(BaseConf.AccessLog.LogLevel)
	return nil
}

//公共销毁函数
func Destroy() {
	DB.Close()
}

func parseConfig(path string, conf interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config %v fail, %v", path, err)
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read config %v fail, %v", path, err)
	}
	err = json.Unmarshal(data, conf)
	if err != nil {
		return fmt.Errorf("unmarshal config %v fail, %v", path, err)
	}
	return nil
}
