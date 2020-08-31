package entity

import (
	"github.com/jinzhu/gorm"
)

type GatewayModuleBase struct {
	ID           int64  `json:"id" toml:"-" orm:"column(id);auto" description:"自增主键"`
	LoadType     string `json:"load_type" toml:"load_type" validate:"" orm:"column(load_type);size(255)" description:"负载类型 http/tcp"`
	Name         string `json:"name" toml:"name" validate:"required" orm:"column(name);size(255)" description:"模块名"`
	ServiceName  string `json:"service_name" toml:"service_name" validate:"" orm:"column(service_name);size(255)" description:"服务名称"`
	PassAuthType int8   `json:"pass_auth_type" toml:"pass_auth_type" validate:"" orm:"column(pass_auth_type)" description:"认证传参类型"`
	FrontendAddr string `json:"frontend_addr" toml:"frontend_addr" validate:"" orm:"column(frontend_addr);size(255)" description:"前端绑定ip地址"`
}

func (e *GatewayModuleBase) TableName() string {
	return "gateway_module_base"
}

func (e *GatewayModuleBase) GetAll(db *gorm.DB, orders ...string) ([]*GatewayModuleBase, error) {
	var modules []*GatewayModuleBase
	for _, order := range orders {
		db = db.Order(order)
	}
	err := db.Find(&modules).Error
	return modules, err
}

func (e *GatewayModuleBase) FindByName(db *gorm.DB, name string) (*GatewayModuleBase, error) {
	var modules GatewayModuleBase
	err := db.Where("name = ?", name).First(&modules).Error
	if err == gorm.ErrRecordNotFound {
		return &modules, nil
	}
	return &modules, err
}

func (e *GatewayModuleBase) FindByPort(db *gorm.DB, port string) (*GatewayModuleBase, error) {
	var modules GatewayModuleBase
	err := db.Where("frontend_addr = ?", port).First(&modules).Error
	if err == gorm.ErrRecordNotFound {
		return &modules, nil
	}
	return &modules, err
}

func (e *GatewayModuleBase) Save(db *gorm.DB) error {
	return db.Save(e).Error
}

func (e *GatewayModuleBase) Del(db *gorm.DB) error {
	if err := db.Where("id = ?", e.ID).Delete(e).Error; err != nil {
		return err
	}
	return nil
}

func (e *GatewayModuleBase) GetPk() int64 {
	return e.ID
}
