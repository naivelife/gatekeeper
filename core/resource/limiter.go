package resource

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var Limiters *LimiterManager

// 限流器管理器
type LimiterManager struct {
	sync.RWMutex
	limiters map[string]*Limiter
}

// 限流器
type Limiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewLimiterManager() *LimiterManager {
	lm := &LimiterManager{
		limiters: make(map[string]*Limiter),
	}
	go lm.CleanupLimiter()
	return lm
}

// 创建一个限流器
// @param name 对于租户，name为appID；对于服务，name为module_name
func (t *LimiterManager) NewLimiter(name string, qps int64) *rate.Limiter {
	limiter := rate.NewLimiter(rate.Limit(qps), int(qps*3))
	t.Lock()
	t.limiters[name] = &Limiter{limiter, time.Now()}
	t.Unlock()
	return limiter
}

// 获取给定key对应的限流器,不存在就创建
// @param name 对于租户，name为appID；对于服务，name为module_name
func (t *LimiterManager) GetLimiter(name string, qps int64) *rate.Limiter {
	t.RLock()
	v, exists := t.limiters[name]
	if !exists {
		t.RUnlock()
		return t.NewLimiter(name, qps)
	}
	v.lastSeen = time.Now()
	t.RUnlock()
	return v.limiter
}

// 定时清空限流器
func (t *LimiterManager) CleanupLimiter() {
	for {
		time.Sleep(time.Minute)
		t.Lock()
		for k, v := range t.limiters {
			if time.Now().Sub(v.lastSeen) > 5*time.Second {
				delete(t.limiters, k)
			}
		}
		t.Unlock()
	}
}
