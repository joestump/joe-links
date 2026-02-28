// Governing: SPEC-0001 REQ "CLI Entrypoint", "OIDC-Only Authentication", "Server-Side Sessions", ADR-0003, ADR-0004
// Governing: SPEC-0017 REQ "LLM Provider Configuration", ADR-0017
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
	AdminGroups     []string // OIDC group names that grant the admin role
	GroupsClaim     string   // OIDC claim name containing the user's groups (default: "groups")
	ShortKeyword    string   // override the short keyword prefix (default: first label of HTTP host)
	SessionLifetime time.Duration
	InsecureCookies bool
	LLM             struct {
		Provider string // "anthropic", "openai", or "openai-compatible"; empty = disabled
		APIKey   string
		Model    string
		BaseURL  string // override for openai-compatible providers
		Prompt   string // custom prompt template text (overrides built-in default)
	}
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
	cfg.InsecureCookies = v.GetBool("insecure_cookies")
	if raw := v.GetString("oidc.admin_groups"); raw != "" {
		for _, g := range strings.Split(raw, ",") {
			if g = strings.TrimSpace(g); g != "" {
				cfg.AdminGroups = append(cfg.AdminGroups, g)
			}
		}
	}
	cfg.GroupsClaim = v.GetString("oidc.groups_claim")
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	cfg.ShortKeyword = v.GetString("short_keyword")

	cfg.LLM.Provider = v.GetString("llm.provider")
	cfg.LLM.APIKey = v.GetString("llm.api_key")
	cfg.LLM.Model = v.GetString("llm.model")
	cfg.LLM.BaseURL = v.GetString("llm.base_url")
	cfg.LLM.Prompt = v.GetString("llm.prompt")

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
