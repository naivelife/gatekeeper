package entity

import (
	"github.com/jinzhu/gorm"
)

type GatewayAccessControl struct {
	ID              int64  `json:"id" toml:"-" orm:"column(id);auto" description:"自增主键"`
	ModuleID        int64  `json:"module_id" toml:"-" orm:"column(module_id)" description:"模块id"`
	BlackList       string `json:"black_list" toml:"black_list" orm:"column(black_list);size(1000)" description:"黑名单ip"`
	WhiteList       string `json:"white_list" toml:"white_list" orm:"column(white_list);size(1000)" description:"白名单ip"`
	WhiteHostName   string `json:"white_host_name" toml:"white_host_name" orm:"column(white_host_name);size(1000)" description:"白名单主机"`
	AuthType        string `json:"auth_type" toml:"auth_type" orm:"column(auth_type);size(100)" description:"认证方法"`
	ClientFlowLimit int64  `json:"client_flow_limit" toml:"client_flow_limit" orm:"column(client_flow_limit);size(100)" description:"客户端ip限流"`
	Open            int64  `json:"open" toml:"open" orm:"column(open);size(100)" description:"是否开启权限功能"`
}

func (e *GatewayAccessControl) TableName() string {
	return "gateway_access_control"
}

// 根据module获取对应记录
func (e *GatewayAccessControl) GetByModule(db *gorm.DB, moduleID int64) (*GatewayAccessControl, error) {
	var rules []*GatewayAccessControl
	err := db.Model(&GatewayAccessControl{}).
		Where(&GatewayAccessControl{ModuleID: moduleID}).
		Find(&rules).Error
	if len(rules) == 0 {
		return nil, nil
	}
	return rules[0], err
}

func (e *GatewayAccessControl) GetAll(db *gorm.DB) ([]*GatewayAccessControl, error) {
	var rules []*GatewayAccessControl
	err := db.Model(&GatewayAccessControl{}).
		Find(&rules).Error
	return rules, err
}

func (e *GatewayAccessControl) Save(db *gorm.DB) error {
	return db.Save(e).Error
}

func (e *GatewayAccessControl) Del(db *gorm.DB) error {
	if err := db.Where("module_id = ?", e.ModuleID).Delete(e).Error; err != nil {
		return err
	}
	return nil
}

func (e *GatewayAccessControl) GetPk() int64 {
	return e.ID
}
