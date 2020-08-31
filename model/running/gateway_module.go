package running

import "gatekeeper/model/entity"

type GatewayModule struct {
	Base          *entity.GatewayModuleBase    `json:"base" validate:"required" toml:"base"`
	MatchRule     *entity.GatewayMatchRule     `json:"match_rule" validate:"required"  toml:"match_rule"`
	LoadBalance   *entity.GatewayLoadBalance   `json:"load_balance" validate:"required" toml:"load_balance"`
	AccessControl *entity.GatewayAccessControl `json:"access_control" toml:"access_control"`
}
