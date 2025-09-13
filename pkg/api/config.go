package api

import "github.com/defeedco/defeed/pkg/api/auth"

type Config struct {
	Host       string `env:"SERVER_HOST,default=localhost"`
	Port       uint16 `env:"SERVER_PORT,default=8080"`
	Proxied    bool   `env:"SERVER_PROXIED,default=false"`
	AssetsPath string `env:"SERVER_ASSETS_PATH,default=./assets"`
	BaseURL    string `env:"SERVER_BASE_URL,default=/"`
	FaviconURL string `env:"SERVER_FAVICON_URL,default="`
	// CORSOrigin is a comma-separated list of origins.
	CORSOrigin string      `env:"CORS_ORIGIN,default=*"`
	Auth       auth.Config `env:""`
}
