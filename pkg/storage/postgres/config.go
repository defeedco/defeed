package postgres

import (
	"fmt"
	"net"
	"strconv"
)

type Config struct {
	Host        string `env:"DB_HOST,required"`
	User        string `env:"DB_USER,required"`
	Password    string `env:"DB_PASSWORD,required"`
	Name        string `env:"DB_NAME,required"`
	Port        int    `env:"DB_PORT,required"`
	AutoMigrate bool   `env:"DB_AUTO_MIGRATE,default=false"`
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		c.User,
		c.Password,
		net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		c.Name,
	)
}
