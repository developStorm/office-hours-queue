package config

import (
	"fmt"
	"os"

	env "github.com/Netflix/go-env"
)

// Config holds all configuration for the application
type Config struct {
	// Database configuration
	DBUrl      string `env:"QUEUE_DB_URL"`
	DBDatabase string `env:"QUEUE_DB_DATABASE"`
	DBUsername string `env:"QUEUE_DB_USERNAME"`
	DBPassword string

	// OAuth/OIDC configuration
	OIDCIssuerURL      string `env:"QUEUE_OIDC_ISSUER_URL"`
	OAuth2ClientID     string `env:"QUEUE_OAUTH2_CLIENT_ID"`
	OAuth2ClientSecret string
	OAuth2RedirectURI  string `env:"QUEUE_OAUTH2_REDIRECT_URI"`
	OAuth2UsePKCE      bool   `env:"QUEUE_OAUTH2_USE_PKCE" envDefault:"true"`
	ValidDomain        string `env:"QUEUE_VALID_DOMAIN"`

	// Server configuration
	BaseURL          string `env:"QUEUE_BASE_URL"`
	UseSecureCookies bool   `env:"USE_SECURE_COOKIES" envDefault:"false"`

	// Secret file paths
	DBPasswordFile         string `env:"QUEUE_DB_PASSWORD_FILE" envDefault:"deploy/secrets/postgres_password"`
	OAuth2ClientSecretFile string `env:"QUEUE_OAUTH2_CLIENT_SECRET_FILE" envDefault:"deploy/secrets/oauth2_client_secret"`
	SessionsKeyFile        string `env:"QUEUE_SESSIONS_KEY_FILE" envDefault:"deploy/secrets/signing.key"`
	MetricsPasswordFile    string `env:"METRICS_PASSWORD_FILE" envDefault:"deploy/secrets/metrics_password"`

	// Secret file contents
	SessionsKey     []byte
	MetricsPassword string

	// Extras - any environment variables that weren't matched will be here
	Extras env.EnvSet
}

// Global application configuration
var AppConfig Config

// Load loads configuration from environment variables and secret files
func Load() error {
	// Parse environment variables
	es, err := env.UnmarshalFromEnviron(&AppConfig)
	if err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Store extras
	AppConfig.Extras = es

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
