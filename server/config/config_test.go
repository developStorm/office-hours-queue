package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecretWhitespace(t *testing.T) {
	for _, input := range []string{"secret value", " \t\r\nsecret value\r\n\t ", "\u2003secret value\u2003", " \t\r\n"} {
		t.Run(input, func(t *testing.T) {
			previous := AppConfig
			t.Cleanup(func() { AppConfig = previous })
			AppConfig = Config{}
			for name, value := range map[string]string{
				"QUEUE_DB_URL": "localhost", "QUEUE_DB_DATABASE": "queue", "QUEUE_DB_USERNAME": "queue",
				"QUEUE_OIDC_ISSUER_URL": "https://example.com", "QUEUE_OAUTH2_CLIENT_ID": "queue",
				"QUEUE_OAUTH2_REDIRECT_URI": "https://example.com/callback", "QUEUE_VALID_DOMAIN": "example.com",
			} {
				t.Setenv(name, value)
			}
			dir := t.TempDir()
			key := []byte{'\n', 0, 255, ' ', '\t'}
			for name, contents := range map[string][]byte{
				"QUEUE_DB_PASSWORD_FILE":          []byte(input),
				"QUEUE_OAUTH2_CLIENT_SECRET_FILE": []byte(input),
				"METRICS_PASSWORD_FILE":           []byte(input),
				"QUEUE_SESSIONS_KEY_FILE":         key,
			} {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, contents, 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv(name, path)
			}
			if err := Load(); err != nil {
				t.Fatal(err)
			}
			want := "secret value"
			if input == " \t\r\n" {
				want = ""
			}
			for name, got := range map[string]string{
				"database": AppConfig.DBPassword,
				"oauth":    AppConfig.OAuth2ClientSecret,
				"metrics":  AppConfig.MetricsPassword,
			} {
				if got != want {
					t.Errorf("%s secret = %q, want %q", name, got, want)
				}
			}
			if !bytes.Equal(AppConfig.SessionsKey, key) {
				t.Error("binary session key was altered")
			}
		})
	}
}
