package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
	"github.com/jmoiron/sqlx"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

const RequestIDContextKey = "request_id"
const loggerContextKey = "logger"

func ksuidInserter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ksuid.New()
		ctx := context.WithValue(r.Context(), RequestIDContextKey, id)
		w.Header().Add("X-Request-ID", id.String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) realIPOrFail(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip, _, _ := strings.Cut(xff, ",")
			r.RemoteAddr = ip
			next.ServeHTTP(w, r)
			return
		}

		s.getCtxLogger(r).Warnw("missing X-Forwarded-For header, app must be behind a proxy in production",
			"remote_addr", r.RemoteAddr,
		)
		s.internalServerError(w, r)
	})
}

type transactioner interface {
	BeginTx() (*sqlx.Tx, error)
}

const (
	RequestErrorContextKey = "request_error"
	TransactionContextKey  = "transaction"
)

// This function does tie the API package to sqlx to an extent, but it
// doesn't need to be used in tests (individual handlers can still be
// unit tested without this middleware, since the transaction is passed
// through transparently in the context). I'm not advocating that this is
// the cleanest pattern, but we definitely need to get transactions into
// each request.
func (s *Server) transaction(tr transactioner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tx, err := tr.BeginTx()
			if err != nil {
				s.getCtxLogger(r).Errorw("failed to begin DB transaction", "err", err)
				s.internalServerError(w, r)
				return
			}

			// Yes, this is a pointer to an interface. Yes, having handlers
			// propogate information back up via context is probably not the
			// best pattern, but go-chi doesn't directly support handlers and
			// middleware returning errors, and this only needs to occur in one
			// other place (E.ServeHTTP).
			ctx := context.WithValue(r.Context(), RequestErrorContextKey, &err)
			ctx = context.WithValue(ctx, TransactionContextKey, tx)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)

			// err might have been mutated by the handler since we passed the
			// context a pointer to it.
			if err != nil {
				err = tx.Rollback()
				// The handler already wrote a status code, so the best we can
				// do is log the failed rollback.
				if err != nil {
					s.getCtxLogger(r).Errorw("transaction rollback failed", "err", err)
				}
				return
			}

			err = tx.Commit()
			if err != nil {
				// The handler already wrote a status code, so the best we can
				// do is log the failed commit.
				s.getCtxLogger(r).Errorw("transaction commit failed", "err", err)
			}
		})
	}
}

func (s *Server) sessionRetriever(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.sessions.Get(r, "session")
		if err != nil {
			s.getCtxLogger(r).Infow("got invalid session", "err", err)
			http.SetCookie(w, emptySessionCookie)
			s.errorMessage(
				http.StatusUnauthorized,
				"Try logging in again.",
				w, r,
			)
			return
		}

		email, ok := session.Values["email"].(string)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		name, ok := session.Values["name"].(string)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		firstName, ok := session.Values["first_name"].(string)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		groups, ok := session.Values["groups"].([]string)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), emailContextKey, email)
		ctx = context.WithValue(ctx, nameContextKey, name)
		ctx = context.WithValue(ctx, firstNameContextKey, firstName)
		ctx = context.WithValue(ctx, sessionContextKey, session.Values)
		ctx = context.WithValue(ctx, GroupsContextKey, groups)
		ctx = context.WithValue(ctx, loggerContextKey, s.getCtxLogger(r).With("email", email))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if msg := recover(); msg != nil {
				s.getCtxLogger(r).Errorw("recovered panic",
					"panic_message", msg,
				)
				s.internalServerError(w, r)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

type siteAdmin interface {
	SiteAdmin(ctx context.Context, email string) (bool, error)
}

func (s *Server) EnsureSiteAdmin(sa siteAdmin, shouldLog bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := r.Context().Value(emailContextKey).(string)
			admin, err := sa.SiteAdmin(r.Context(), email)
			if err != nil || !admin {
				s.getCtxLogger(r).Warnw("non-admin attempting to access resource requiring site admin")
				s.errorMessage(
					http.StatusForbidden,
					"You're not supposed to be here. :)",
					w, r,
				)
				return
			}

			if shouldLog {
				s.getCtxLogger(r).Infow("entering site admin context")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setupCtxLogger adds consistent logging fields to all requests
func (s *Server) setupCtxLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxLogger := s.logger.With(
			RequestIDContextKey, r.Context().Value(RequestIDContextKey),
			"trace_id", r.RemoteAddr,
		)
		ctx := context.WithValue(r.Context(), loggerContextKey, ctxLogger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getCtxLogger is a helper function to retrieve the enhanced logger from context
func (s *Server) getCtxLogger(r *http.Request) *zap.SugaredLogger {
	if logger, ok := r.Context().Value(loggerContextKey).(*zap.SugaredLogger); ok {
		return logger
	}

	// Fallback to the default logger if not found in context (shouldn't happen?)
	return s.logger.With("fallback_logger", "true")
}

// limitHandler is called when the rate limit is exceeded
func (s *Server) getRateLimitOpts() []httprate.Option {
	return []httprate.Option{
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			s.errorMessage(
				http.StatusTooManyRequests,
				"Whoooa slow down! You're making too many requests.",
				w, r,
			)
		}),
		httprate.WithResponseHeaders(httprate.ResponseHeaders{Reset: "X-RateLimit-Reset"}),
	}
}

func (s *Server) rateLimiter(rate int, window time.Duration) func(http.Handler) http.Handler {
	rl := httprate.NewRateLimiter(rate, window, s.getRateLimitOpts()...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, ok := r.Context().Value(emailContextKey).(string)
			if !ok || key == "" {
				key = r.RemoteAddr
			}

			if rl.RespondOnLimit(w, r, key) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
