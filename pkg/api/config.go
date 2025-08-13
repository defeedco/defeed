package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glanceapp/glance/web"
)

type Config struct {
	Host       string `env:"SERVER_HOST,default=localhost"`
	Port       uint16 `env:"SERVER_PORT,default=8080"`
	Proxied    bool   `env:"SERVER_PROXIED,default=false"`
	AssetsPath string `env:"SERVER_ASSETS_PATH,default=./assets"`
	BaseURL    string `env:"SERVER_BASE_URL,default=/"`
	FaviconURL string `env:"SERVER_FAVICON_URL,default="`
	CORSOrigin string `env:"CORS_ORIGIN,default=*"`

	createdAt time.Time
}

func NewDefaultConfig() Config {
	c := Config{
		Host:       "localhost",
		Port:       8080,
		Proxied:    false,
		AssetsPath: "./assets",
		BaseURL:    "/",
		FaviconURL: "",
		CORSOrigin: "*",
		createdAt:  time.Now(),
	}

	c.FaviconURL = c.StaticAssetPath("favicon.png")

	return c
}

func (c *Config) Init() {
	if c.FaviconURL == "" {
		c.FaviconURL = c.StaticAssetPath("favicon.png")
	}
}

func (c *Config) FaviconType() string {
	if strings.HasSuffix(c.FaviconURL, ".svg") {
		return "image/svg+xml"
	} else {
		return "image/png"
	}
}

func (c *Config) resolveUserDefinedAssetPath(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return c.BaseURL + path
	}

	return path
}

func (c *Config) StaticAssetPath(asset string) string {
	return c.BaseURL + "static/" + web.StaticFSHash + "/" + asset
}

func (c *Config) VersionedAssetPath(asset string) string {
	return c.BaseURL + asset +
		"?v=" + strconv.FormatInt(c.createdAt.Unix(), 10)
}

func (c *Config) validate() error {
	if c.AssetsPath != "" {
		if _, err := os.Stat(c.AssetsPath); os.IsNotExist(err) {
			return fmt.Errorf("assets directory does not exist: %s", c.AssetsPath)
		}
	}

	return nil
}
