package running

type Modules struct {
	Module map[string]*GatewayModule `json:"module" toml:"module" validate:"required"`
}
