package entity

import (
	"github.com/jinzhu/gorm"
)

type GatewayMatchRule struct {
	ID         int64  `json:"id" toml:"-" orm:"column(id);auto" description:"自增主键"`
	ModuleID   int64  `json:"module_id" toml:"-" orm:"column(module_id)" description:"模块id"`
	Type       string `json:"type" toml:"type" validate:"required" orm:"column(type)" description:"匹配类型"`
	Rule       string `json:"rule" toml:"rule" validate:"required" orm:"column(rule);size(1000)" description:"规则"`
	RuleExt    string `json:"rule_ext" validate:"required" toml:"rule_ext" orm:"column(rule_ext);size(1000)" description:"拓展规则"`
	URLRewrite string `json:"url_rewrite" validate:"required" toml:"url_rewrite" orm:"column(rule_ext);size(1000)" description:"url重写"`
}

func (o *GatewayMatchRule) TableName() string {
	return "gateway_match_rule"
}

func (o *GatewayMatchRule) GetAll(db *gorm.DB) ([]*GatewayMatchRule, error) {
	var rules []*GatewayMatchRule
	err := db.Find(&rules).Error
	return rules, err
}

func (o *GatewayMatchRule) GetByModule(db *gorm.DB, moduleID int64) ([]*GatewayMatchRule, error) {
	var rules []*GatewayMatchRule
	err := db.Where(&GatewayMatchRule{ModuleID: moduleID}).
		Find(&rules).Error
	return rules, err
}

func (o *GatewayMatchRule) Save(db *gorm.DB) error {
	return db.Save(o).Error
}

func (o *GatewayMatchRule) FindByURLPrefix(db *gorm.DB, prefix string) (*GatewayMatchRule, error) {
	var rule GatewayMatchRule
	err := db.Where("rule = ?", prefix).First(&rule).Error
	if err == gorm.ErrRecordNotFound {
		return &rule, nil
	}
	return &rule, err
}

func (o *GatewayMatchRule) Del(db *gorm.DB) error {
	if err := db.Where("module_id = ?", o.ModuleID).Delete(o).Error; err != nil {
		return err
	}
	return nil
}

func (o *GatewayMatchRule) GetPk() int64 {
	return o.ID
}
