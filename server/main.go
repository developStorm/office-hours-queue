package main

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/CarsonHoffman/office-hours-queue/server/api"
	"github.com/CarsonHoffman/office-hours-queue/server/config"
	"github.com/CarsonHoffman/office-hours-queue/server/db"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func main() {
	z, _ := zap.NewProduction()
	l := z.Sugar().With("name", "queue")

	// Load configuration from environment
	if err := config.Load(); err != nil {
		l.Fatalw("failed to load configuration", "err", err)
	}

	// Initialize OIDC provider
	provider, err := oidc.NewProvider(context.Background(), config.AppConfig.OIDCIssuerURL)
	if err != nil {
		l.Fatalw("failed to create OIDC provider", "err", err)
	}

	oauthConfig := oauth2.Config{
		Endpoint:     provider.Endpoint(),
		ClientID:     config.AppConfig.OAuth2ClientID,
		ClientSecret: config.AppConfig.OAuth2ClientSecret,
		RedirectURL:  config.AppConfig.OAuth2RedirectURI,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile", "groups"},
	}

	// Initialize database
	db, err := db.New(
		config.AppConfig.DBUrl,
		config.AppConfig.DBDatabase,
		config.AppConfig.DBUsername,
		config.AppConfig.DBPassword,
	)
	if err != nil {
		l.Fatalw("failed to set up database", "err", err)
	}

	// Initialize API server
	s := api.New(db, l, db.DB.DB, provider, oauthConfig)

	r := chi.NewRouter()
	r.Mount("/", s)

	go func() {
		d := chi.NewRouter()

		d.Get("/debug/pprof/*", pprof.Index)
		d.Get("/debug/pprof/cmdline", pprof.Cmdline)
		d.Get("/debug/pprof/profile", pprof.Profile)
		d.Get("/debug/pprof/symbol", pprof.Symbol)
		d.Get("/debug/pprof/trace", pprof.Trace)

		l.Fatalw("pprof server failed", "err", http.ListenAndServe(":6060", d))
	}()

	l.Fatalw("http server failed", "err", http.ListenAndServe(":8080", r))
}
