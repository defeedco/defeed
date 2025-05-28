package api

import (
	"fmt"
	"github.com/glanceapp/glance/web"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName    string `yaml:"app-name"`
	Host       string `yaml:"host"`
	Port       uint16 `yaml:"port"`
	Proxied    bool   `yaml:"proxied"`
	AssetsPath string `yaml:"assets-path"`
	BaseURL    string `yaml:"base-url"`
	FaviconURL string `yaml:"favicon-url"`

	createdAt time.Time
}

func NewDefaultConfig() Config {
	c := Config{
		AppName:    "Pulse",
		Host:       "localhost",
		Port:       8080,
		Proxied:    false,
		AssetsPath: "./assets",
		BaseURL:    "/",
		FaviconURL: "",
		createdAt:  time.Now(),
	}

	c.FaviconURL = c.StaticAssetPath("favicon.png")

	return c
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
