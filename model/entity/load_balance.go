package entity

import (
	"github.com/jinzhu/gorm"
)

type GatewayLoadBalance struct {
	ID            int64  `json:"id" toml:"-" orm:"column(id);auto" description:"自增主键"`
	ModuleID      int64  `json:"module_id" toml:"-" orm:"column(module_id)"`
	CheckMethod   string `json:"check_method" validate:"required" toml:"check_method" orm:"column(check_method);size(200)" description:"检查方法"`
	CheckURL      string `json:"check_url" validate:"" toml:"check_url" orm:"column(check_url);size(500)" description:"检测url"`
	CheckTimeout  int    `json:"check_timeout" validate:"required,min=100" toml:"check_timeout" orm:"column(check_timeout);size(500)" description:"检测超时时间"`
	CheckInterval int    `json:"check_interval" validate:"required,min=100" toml:"check_interval" orm:"column(check_interval);size(500)" description:"检测url"`

	Type                string `json:"type" validate:"required" toml:"type" orm:"column(type);size(100)" description:"轮询方式"`
	IPList              string `json:"ip_list" validate:"required" toml:"ip_list" orm:"column(ip_list);size(500)" description:"ip列表"`
	WeightList          string `json:"weight_list" validate:"" toml:"weight_list" orm:"column(weight_list);size(500)" description:"ip列表"`
	ForbidList          string `json:"forbid_list" validate:"" toml:"forbid_list" orm:"column(forbid_list);size(1000)" description:"禁用 ip列表"`
	ProxyConnectTimeout int    `json:"proxy_connect_timeout" validate:"required,min=1" toml:"proxy_connect_timeout" orm:"column(proxy_connect_timeout)" description:"单位ms，连接后端超时时间"`
	ProxyHeaderTimeout  int    `json:"proxy_header_timeout" validate:"" toml:"proxy_header_timeout" orm:"column(proxy_header_timeout)" description:"单位ms，后端服务器数据回传时间"`
	ProxyBodyTimeout    int    `json:"proxy_body_timeout" validate:"" toml:"proxy_body_timeout" orm:"column(proxy_body_timeout)" description:"单位ms，后端服务器响应时间"`
	MaxIdleConn         int    `json:"max_idle_conn" validate:"" toml:"max_idle_conn" orm:"column(max_idle_conn)"`
	IdleConnTimeout     int    `json:"idle_conn_timeout" validate:"" toml:"idle_conn_timeout" orm:"column(idle_conn_timeout)" description:"keep-alived超时时间，新增"`
}

func (o *GatewayLoadBalance) TableName() string {
	return "gateway_load_balance"
}

func (o *GatewayLoadBalance) GetAll(db *gorm.DB) ([]*GatewayLoadBalance, error) {
	var rules []*GatewayLoadBalance
	err := db.Model(&GatewayLoadBalance{}).
		Find(&rules).Error
	return rules, err
}

func (o *GatewayLoadBalance) GetByModule(db *gorm.DB, moduleID int64) (*GatewayLoadBalance, error) {
	var rules []*GatewayLoadBalance
	err := db.Model(&GatewayLoadBalance{}).
		Where(&GatewayLoadBalance{ModuleID: moduleID}).
		Find(&rules).Error
	if len(rules) == 0 {
		return nil, nil
	}
	return rules[0], err
}

func (o *GatewayLoadBalance) Save(db *gorm.DB) error {
	return db.Save(o).Error
}

func (o *GatewayLoadBalance) Del(db *gorm.DB) error {
	if err := db.Where("module_id = ?", o.ModuleID).Delete(o).Error; err != nil {
		return err
	}
	return nil
}

func (o *GatewayLoadBalance) GetPk() int64 {
	return o.ID
}
