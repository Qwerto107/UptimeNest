// Package config 从环境变量加载运行配置，不将敏感连接信息写入仓库。
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Address       string
	DBType        string
	DSN           string
	WebDir        string
	CookieSecure  bool
	Workers       int
	SetupToken    string
	SecretKeyFile string
}

func Load() (Config, error) {
	c := Config{Address: value("UPTIMENEST_ADDRESS", "127.0.0.1:8090"), DBType: value("UPTIMENEST_DB_TYPE", "sqlite"), DSN: os.Getenv("UPTIMENEST_DB_DSN"), WebDir: os.Getenv("UPTIMENEST_WEB_DIR"), Workers: 32}
	c.SecretKeyFile = value("UPTIMENEST_SECRET_KEY_FILE", "data/secret.key")
	if c.DBType != "sqlite" && c.DBType != "postgres" {
		return c, fmt.Errorf("UPTIMENEST_DB_TYPE 仅支持 sqlite 或 postgres")
	}
	if c.DSN == "" {
		if c.DBType == "postgres" {
			return c, fmt.Errorf("PostgreSQL 必须设置 UPTIMENEST_DB_DSN")
		}
		c.DSN = "data/uptimenest.db"
	}
	if v := os.Getenv("UPTIMENEST_COOKIE_SECURE"); v != "" {
		secure, err := strconv.ParseBool(v)
		if err != nil {
			return c, fmt.Errorf("UPTIMENEST_COOKIE_SECURE 必须为布尔值")
		}
		c.CookieSecure = secure
	}
	return c, nil
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
