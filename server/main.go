package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/CarsonHoffman/office-hours-queue/server/api"
	"github.com/CarsonHoffman/office-hours-queue/server/db"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func main() {
	z, _ := zap.NewProduction()
	l := z.Sugar().With("name", "queue")

	password, err := ioutil.ReadFile(os.Getenv("QUEUE_DB_PASSWORD_FILE"))
	if err != nil {
		l.Fatalw("failed to load DB password file", "err", err)
	}

	oauthClientSecret, err := ioutil.ReadFile(os.Getenv("QUEUE_OAUTH2_CLIENT_SECRET_FILE"))
	if err != nil {
		l.Fatalw("failed to load OAuth2 client secret file", "err", err)
	}

	provider, err := oidc.NewProvider(context.Background(), os.Getenv("QUEUE_OIDC_ISSUER_URL"))
	if err != nil {
		l.Fatalw("failed to create OIDC provider", "err", err)
	}

	config := oauth2.Config{
		Endpoint:     provider.Endpoint(),
		ClientID:     os.Getenv("QUEUE_OAUTH2_CLIENT_ID"),
		ClientSecret: string(oauthClientSecret),
		RedirectURL:  os.Getenv("QUEUE_OAUTH2_REDIRECT_URI"),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	db, err := db.New(os.Getenv("QUEUE_DB_URL"), os.Getenv("QUEUE_DB_DATABASE"), os.Getenv("QUEUE_DB_USERNAME"), string(password))
	if err != nil {
		l.Fatalw("failed to set up database", "err", err)
	}

	s := api.New(db, l, db.DB.DB, provider, config)

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
