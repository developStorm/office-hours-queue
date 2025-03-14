package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/dchest/uniuri"
)

const (
	emailContextKey     = "email"
	nameContextKey      = "name"
	firstNameContextKey = "first_name"
	sessionContextKey   = "session"
	stateLength         = 64
)

var emptySessionCookie = &http.Cookie{
	Name:     "session",
	Value:    "",
	MaxAge:   -1,
	HttpOnly: true,
	Secure:   os.Getenv("USE_SECURE_COOKIES") == "true",
	Path:     "/",
}

func (s *Server) ValidLoginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.sessions.Get(r, "session")
		if err != nil {
			s.logger.Infow("got invalid session",
				RequestIDContextKey, r.Context().Value(RequestIDContextKey),
				"err", err,
			)
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
			s.errorMessage(
				http.StatusUnauthorized,
				"Come back with a login!",
				w, r,
			)
			return
		}

		domain := os.Getenv("QUEUE_VALID_DOMAIN")
		if !strings.HasSuffix(email, "@"+domain) {
			s.logger.Warnw("found valid session with email outside valid domain",
				RequestIDContextKey, r.Context().Value(RequestIDContextKey),
				"valid_domain", domain,
				"email", email,
			)
			s.errorMessage(
				http.StatusUnauthorized,
				"Oh dear, it looks like you don't have an @"+os.Getenv("QUEUE_VALID_DOMAIN")+" account.",
				w, r,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) FowardAuth() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		s.logger.Infow("forward auth passed",
			"X-Forwarded-Uri", r.Header.Get("X-Forwarded-Uri"),
			"email", r.Context().Value(emailContextKey).(string),
		)
		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

func (s *Server) OAuth2LoginLink() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		session, err := s.sessions.New(r, "session")
		if err != nil {
			s.logger.Errorw("got invalid session on login",
				RequestIDContextKey, r.Context().Value(RequestIDContextKey),
				"err", err,
			)
			http.SetCookie(w, emptySessionCookie)
			http.Redirect(w, r, s.baseURL+"api/oauth2login", http.StatusTemporaryRedirect)
			return nil
		}

		state := uniuri.NewLen(64)
		session.Values["state"] = state
		s.sessions.Save(r, w, session)

		url := s.oauthConfig.AuthCodeURL(state)

		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return nil
	}
}

func (s *Server) OAuth2Callback() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		l := s.logger.With(RequestIDContextKey, r.Context().Value(RequestIDContextKey))
		code := r.FormValue("code")
		state := r.FormValue("state")

		session, err := s.sessions.Get(r, "session")
		if err != nil {
			l.Errorw("got invalid session on login", "err", err)
			http.SetCookie(w, emptySessionCookie)
			http.Redirect(w, r, s.baseURL+"api/oauth2login", http.StatusTemporaryRedirect)
			return nil
		}

		savedState, ok := session.Values["state"].(string)
		if !ok {
			l.Errorw("failed to get state from session", "err", err)
			return err
		}

		if state != savedState {
			l.Warnw("state doesn't match stored state", "received", state, "expected", savedState)
			return StatusError{
				http.StatusUnauthorized,
				"Something went really wrong.",
			}
		}

		token, err := s.oauthConfig.Exchange(r.Context(), code)
		if err != nil {
			l.Errorw("failed to exchange token", "err", err)
			return err
		}

		client := s.oauthConfig.Client(r.Context(), token)
		rawInfo, err := client.Get(s.oidcProvider.UserInfoEndpoint())
		if err != nil {
			l.Errorw("failed to get user info", "err", err)
			return err
		}

		var info struct {
			Email     string `json:"email"`
			Name      string `json:"name"`
			GivenName string `json:"given_name"`
		}
		if err := json.NewDecoder(rawInfo.Body).Decode(&info); err != nil {
			l.Errorw("failed to decode user info", "err", err)
			return err
		}

		session.Values["email"] = info.Email
		session.Values["name"] = info.Name
		session.Values["first_name"] = info.GivenName
		s.sessions.Save(r, w, session)

		s.logger.Infow("processed login",
			RequestIDContextKey, r.Context().Value(RequestIDContextKey),
			"email", info.Email,
			"name", info.Name,
		)
		http.Redirect(w, r, s.baseURL, http.StatusTemporaryRedirect)
		return nil
	}
}

func (s *Server) Logout() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		s.logger.Infow("logged out",
			RequestIDContextKey, r.Context().Value(RequestIDContextKey),
			emailContextKey, r.Context().Value(emailContextKey),
		)

		http.SetCookie(w, emptySessionCookie)
		http.Redirect(w, r, s.baseURL, http.StatusTemporaryRedirect)
		return nil
	}
}

type getAdminCourses interface {
	GetAdminCourses(ctx context.Context, email string) ([]string, error)
}

type getUserInfo interface {
	siteAdmin
	getAdminCourses
}

func (s *Server) GetCurrentUserInfo(gi getUserInfo) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		email := r.Context().Value(emailContextKey).(string)

		admin, err := gi.SiteAdmin(r.Context(), email)
		if err != nil {
			s.logger.Errorw("failed to get site admin status",
				RequestIDContextKey, r.Context().Value(RequestIDContextKey),
				"email", email,
				"err", err,
			)
			return err
		}

		courses, err := gi.GetAdminCourses(r.Context(), email)
		if err != nil {
			s.logger.Errorw("failed to get admin courses",
				RequestIDContextKey, r.Context().Value(RequestIDContextKey),
				"email", email,
				"err", err,
			)
			return err
		}

		// If any read or assertion fails, the string will be empty and
		// will get caught by omitempty in the JSON encoding. It looks bad,
		// but is actually not horrible!
		name, _ := r.Context().Value(nameContextKey).(string)
		firstName, _ := r.Context().Value(firstNameContextKey).(string)

		resp := struct {
			Email        string   `json:"email"`
			SiteAdmin    bool     `json:"site_admin"`
			AdminCourses []string `json:"admin_courses"`
			Name         string   `json:"name"`
			FirstName    string   `json:"first_name"`
		}{email, admin, courses, name, firstName}

		return s.sendResponse(http.StatusOK, resp, w, r)
	}
}
