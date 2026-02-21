// Governing: SPEC-0001 REQ "CLI Entrypoint", ADR-0004
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Addr string
	}
	DB struct {
		Driver string
		DSN    string
	}
	OIDC struct {
		Issuer       string
		ClientID     string
		ClientSecret string
		RedirectURL  string
	}
	AdminEmail      string
	SessionLifetime time.Duration
}

// Load reads config from environment (JOE_ prefix) and optional joe-links.yaml.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("JOE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigName("joe-links")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	_ = v.ReadInConfig() // optional config file

	v.SetDefault("http.addr", ":8080")
	v.SetDefault("session.lifetime", "720h")

	cfg := &Config{}
	cfg.HTTP.Addr = v.GetString("http.addr")
	cfg.DB.Driver = v.GetString("db.driver")
	cfg.DB.DSN = v.GetString("db.dsn")
	cfg.OIDC.Issuer = v.GetString("oidc.issuer")
	cfg.OIDC.ClientID = v.GetString("oidc.client_id")
	cfg.OIDC.ClientSecret = v.GetString("oidc.client_secret")
	cfg.OIDC.RedirectURL = v.GetString("oidc.redirect_url")
	cfg.AdminEmail = v.GetString("admin_email")

	lifetime, err := time.ParseDuration(v.GetString("session.lifetime"))
	if err != nil {
		return nil, fmt.Errorf("invalid JOE_SESSION_LIFETIME: %w", err)
	}
	cfg.SessionLifetime = lifetime

	if cfg.DB.Driver == "" {
		return nil, fmt.Errorf("JOE_DB_DRIVER is required (sqlite3, mysql, postgres)")
	}
	if cfg.DB.DSN == "" {
		return nil, fmt.Errorf("JOE_DB_DSN is required")
	}
	if cfg.OIDC.Issuer == "" {
		return nil, fmt.Errorf("JOE_OIDC_ISSUER is required")
	}
	if cfg.OIDC.ClientID == "" {
		return nil, fmt.Errorf("JOE_OIDC_CLIENT_ID is required")
	}
	if cfg.OIDC.ClientSecret == "" {
		return nil, fmt.Errorf("JOE_OIDC_CLIENT_SECRET is required")
	}
	if cfg.OIDC.RedirectURL == "" {
		return nil, fmt.Errorf("JOE_OIDC_REDIRECT_URL is required")
	}

	return cfg, nil
}
