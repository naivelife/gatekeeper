package running

import "gatekeeper/model/entity"

// APPConfigs app列表配置
type Apps struct {
	Apps map[string]*entity.GatewayAPP `json:"apps" toml:"apps" validate:"required"`
}
