package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the application
type Config struct {
	// Database configuration
	DBUrl      string `env:"QUEUE_DB_URL,notEmpty"`
	DBDatabase string `env:"QUEUE_DB_DATABASE,notEmpty"`
	DBUsername string `env:"QUEUE_DB_USERNAME,notEmpty"`
	DBPassword string

	// OAuth/OIDC configuration
	OIDCIssuerURL      string `env:"QUEUE_OIDC_ISSUER_URL,notEmpty"`
	OAuth2ClientID     string `env:"QUEUE_OAUTH2_CLIENT_ID,notEmpty"`
	OAuth2ClientSecret string
	OAuth2RedirectURI  string   `env:"QUEUE_OAUTH2_REDIRECT_URI,notEmpty"`
	OAuth2UsePKCE      bool     `env:"QUEUE_OAUTH2_USE_PKCE" envDefault:"true"`
	ValidDomain        string   `env:"QUEUE_VALID_DOMAIN,notEmpty"`
	SiteAdminGroups    []string `env:"QUEUE_SITE_ADMIN_GROUPS" envSeparator:","`
	siteAdminGroupsSet map[string]struct{}

	// Server configuration
	BaseURL          string `env:"QUEUE_BASE_URL"`
	UseSecureCookies bool   `env:"USE_SECURE_COOKIES" envDefault:"false"`

	// Secret file paths - private to avoid exposing sensitive paths
	DBPasswordFile         string `env:"QUEUE_DB_PASSWORD_FILE,notEmpty"`
	OAuth2ClientSecretFile string `env:"QUEUE_OAUTH2_CLIENT_SECRET_FILE,notEmpty"`
	SessionsKeyFile        string `env:"QUEUE_SESSIONS_KEY_FILE,notEmpty"`
	MetricsPasswordFile    string `env:"METRICS_PASSWORD_FILE,notEmpty"`

	// Secret file contents
	SessionsKey     []byte
	MetricsPassword string
}

// Global application configuration
var AppConfig Config

// AnyInSiteAdminGroups checks if any of the user's groups is an admin group
func (c *Config) AnyInSiteAdminGroups(userGroups []string) bool {
	for _, group := range userGroups {
		if _, ok := c.siteAdminGroupsSet[group]; ok {
			return true
		}
	}
	return false
}

// Load loads configuration from environment variables and secret files
func Load() error {
	// Parse environment variables
	if err := env.Parse(&AppConfig); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Build admin groups set
	AppConfig.siteAdminGroupsSet = make(map[string]struct{})
	for _, group := range AppConfig.SiteAdminGroups {
		group = strings.TrimSpace(group)
		if group != "" {
			AppConfig.siteAdminGroupsSet[group] = struct{}{}
		}
	}

	// Load secrets from files
	dbPassword, err := os.ReadFile(AppConfig.DBPasswordFile)
	if err != nil {
		return fmt.Errorf("failed to load DB password file: %w", err)
	}
	AppConfig.DBPassword = string(dbPassword)

	oauthClientSecret, err := os.ReadFile(AppConfig.OAuth2ClientSecretFile)
	if err != nil {
		return fmt.Errorf("failed to load OAuth2 client secret file: %w", err)
	}
	AppConfig.OAuth2ClientSecret = string(oauthClientSecret)

	sessionsKey, err := os.ReadFile(AppConfig.SessionsKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load sessions key file: %w", err)
	}
	AppConfig.SessionsKey = sessionsKey

	metricsPassword, err := os.ReadFile(AppConfig.MetricsPasswordFile)
	if err != nil {
		return fmt.Errorf("failed to load metrics password file: %w", err)
	}
	AppConfig.MetricsPassword = string(metricsPassword)

	return nil
}
