package resource

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"

	"gatekeeper/config"
	"gatekeeper/constant"
)

var FlowCounters *FlowCounterManager

// 流量统计管理器
type FlowCounterManager struct {
	requestCountMap     map[string]*RequestCountService
	requestCountMapLock sync.RWMutex
	appCountMap         map[string]*APPCountService
	appCountMapLock     sync.RWMutex
}

// request计数结构体
// 针对服务级别的统计
type RequestCountService struct {
	sync.RWMutex
	ModuleName  string
	Interval    time.Duration
	ReqCount    int64
	TotalCount  int64
	QPS         int64
	Unix        int64
	TickerCount int64
	ReqDate     string
}

// app统计结构体
// 针对租户级别的统计
type APPCountService struct {
	sync.RWMutex
	AppID       string
	Interval    time.Duration
	ReqCount    int64
	TotalCount  int64
	QPS         int64
	Unix        int64
	TickerCount int64
	ReqDate     string
}

// 创建流量统计管理器
func NewFlowCounterManager() *FlowCounterManager {
	return &FlowCounterManager{
		requestCountMap:     make(map[string]*RequestCountService),
		requestCountMapLock: sync.RWMutex{},
		appCountMap:         make(map[string]*APPCountService),
		appCountMapLock:     sync.RWMutex{},
	}
}

func NewRequestCountService(moduleName string, interval time.Duration, maxCnt int) (*RequestCountService, error) {
	reqCounter := &RequestCountService{
		ModuleName:  moduleName,
		Interval:    interval,
		ReqCount:    0,
		QPS:         0,
		Unix:        0,
		TickerCount: 0,
		ReqDate:     "",
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// todo，错误打印
			}
		}()
		ticker := time.NewTicker(interval)
		for {
			<-ticker.C
			tickerCount := atomic.LoadInt64(&reqCounter.TickerCount) // 获取数据
			atomic.StoreInt64(&reqCounter.TickerCount, 0)            // 重置数据
			today := time.Now().In(config.TimeLocation).Format("2006-01-02 15:04:05")
			redisKey := constant.RequestModuleCounterPrefix + today + "_" + reqCounter.ModuleName
			todayHour := time.Now().In(config.TimeLocation).Format("2006010215")
			redisHourKey := constant.RequestModuleHourCounterPrefix + todayHour + "_" + reqCounter.ModuleName
			config.RedisPipeline(
				func(c redis.Conn) {
					c.Send("INCRBY", redisKey, tickerCount)
					c.Send("EXPIRE", redisKey, 86400)
					c.Send("INCRBY", redisHourKey, tickerCount)
					c.Send("EXPIRE", redisHourKey, 86400)
				})
			if currentCount, err := redis.Int64(config.RedisDo("GET", redisKey)); err == nil {
				nowUnix := time.Now().Unix()
				nowDate := time.Now().In(config.TimeLocation).Format(constant.DateFormat)
				if reqCounter.ReqDate != nowDate {
					reqCounter.ReqDate = nowDate
					reqCounter.TotalCount = 1
				}
				if reqCounter.Unix == 0 {
					reqCounter.Unix = time.Now().Unix()
				} else {
					if currentCount >= reqCounter.TotalCount && nowUnix > reqCounter.Unix {
						reqCounter.QPS = (currentCount - reqCounter.TotalCount) / (nowUnix - reqCounter.Unix)
						reqCounter.TotalCount = currentCount
						reqCounter.Unix = time.Now().Unix()
					}
				}
			}
		}
	}()
	return reqCounter, nil
}

func NewAPPCountService(appID string, interval time.Duration, maxCnt int) (*APPCountService, error) {
	reqCounter := &APPCountService{
		AppID:       appID,
		Interval:    interval,
		ReqCount:    0,
		QPS:         0,
		Unix:        0,
		TickerCount: 0,
		ReqDate:     "",
	}
	// 每一秒进行一次数据的累加
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// todo，错误打印
			}
		}()
		ticker := time.NewTicker(interval)
		for {
			<-ticker.C
			tickerCount := atomic.LoadInt64(&reqCounter.TickerCount) //获取数据
			atomic.StoreInt64(&reqCounter.TickerCount, 0)            //重置数据

			// todo
			today := time.Now().In(config.TimeLocation).Format("")
			totalAppKey := fmt.Sprintf("%s%s_%s", constant.AccessControlAppIDTotalCallPrefix, today, appID)
			todayHour := time.Now().In(config.TimeLocation).Format("2006010215")
			redisHourKey := fmt.Sprintf("%s%s_%s", constant.AccessControlAppIDHourTotalCallPrefix, todayHour, appID)
			config.RedisPipeline(
				func(c redis.Conn) {
					c.Send("INCRBY", totalAppKey, tickerCount)
					c.Send("EXPIRE", totalAppKey, 86400)
					c.Send("INCRBY", redisHourKey, tickerCount)
					c.Send("EXPIRE", redisHourKey, 86400)
				})
			if currentCount, err := redis.Int64(config.RedisDo("GET", totalAppKey)); err == nil {
				nowUnix := time.Now().Unix()
				nowDate := time.Now().In(config.TimeLocation).Format(constant.DateFormat)
				if reqCounter.ReqDate != nowDate {
					reqCounter.ReqDate = nowDate
					reqCounter.TotalCount = 1
				}
				if reqCounter.Unix == 0 {
					reqCounter.Unix = time.Now().Unix()
				} else {
					if currentCount >= reqCounter.TotalCount && nowUnix > reqCounter.Unix {
						reqCounter.QPS = (currentCount - reqCounter.TotalCount) / (nowUnix - reqCounter.Unix)
						reqCounter.TotalCount = currentCount
						reqCounter.Unix = time.Now().Unix()
					}
				}
			}
		}
	}()
	return reqCounter, nil
}

// 获取一个request模块统计，不存在就创建一个
// 针对服务级别的统计
func (c *FlowCounterManager) GetRequestCounter(moduleName string) *RequestCountService {
	c.requestCountMapLock.RLock()
	if counter, ok := c.requestCountMap[moduleName]; ok {
		c.requestCountMapLock.RUnlock()
		return counter
	}
	c.requestCountMapLock.RUnlock()
	c.requestCountMapLock.Lock()
	defer c.requestCountMapLock.Unlock()
	newCounter, err := NewRequestCountService(moduleName, 1*time.Second, 1)
	if err != nil {
		return nil
	}
	c.requestCountMap[moduleName] = newCounter
	return newCounter
}

func (c *FlowCounterManager) GetAPPCounter(appID string) *APPCountService {
	c.appCountMapLock.RLock()
	if counter, ok := c.appCountMap[appID]; ok {
		c.appCountMapLock.RUnlock()
		return counter
	}
	c.appCountMapLock.RUnlock()
	c.appCountMapLock.Lock()
	defer c.appCountMapLock.Unlock()
	newCounter, err := NewAPPCountService(appID, 1*time.Second, 1)
	if err != nil {
		return nil
	}
	c.appCountMap[appID] = newCounter
	return newCounter
}

func (o *RequestCountService) Increase(ctx context.Context, remoteAddr string) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// todo
			}
		}()
		atomic.AddInt64(&o.TickerCount, 1)
	}()
}

func (o *RequestCountService) GetHourCount(dayHour string) (int64, error) {
	redisKey := constant.RequestModuleHourCounterPrefix + dayHour + "_" + o.ModuleName
	return redis.Int64(config.RedisDo("GET", redisKey))
}

func (o *RequestCountService) GetDayCount(day string) (int64, error) {
	redisKey := constant.RequestModuleCounterPrefix + day + "_" + o.ModuleName
	return redis.Int64(config.RedisDo("GET", redisKey))
}

func (o *APPCountService) Increase() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// todo
			}
		}()
		atomic.AddInt64(&o.TickerCount, 1)
	}()
}

func (o *APPCountService) GetHourCount(dayHour string) (int64, error) {
	redisKey := constant.AccessControlAppIDHourTotalCallPrefix + dayHour + "_" + o.AppID
	return redis.Int64(config.RedisDo("GET", redisKey))
}

func (o *APPCountService) GetDayCount(day string) (int64, error) {
	redisKey := constant.AccessControlAppIDTotalCallPrefix + day + "_" + o.AppID
	return redis.Int64(config.RedisDo("GET", redisKey))
}
