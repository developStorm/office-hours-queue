package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/CarsonHoffman/office-hours-queue/server/config"
	"github.com/dchest/uniuri"
	"golang.org/x/oauth2"
)

const (
	emailContextKey     = "email"
	nameContextKey      = "name"
	firstNameContextKey = "first_name"
	sessionContextKey   = "session"
	GroupsContextKey    = "groups"
	stateLength         = 64
)

var emptySessionCookie = &http.Cookie{
	Name:     "session",
	Value:    "",
	MaxAge:   -1,
	HttpOnly: true,
	Secure:   config.AppConfig.UseSecureCookies,
	Path:     "/",
}

func (s *Server) ValidLoginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(emailContextKey).(string)
		if !ok {
			s.errorMessage(
				http.StatusUnauthorized,
				"Come back with a login!",
				w, r,
			)
			return
		}

		validDomain := config.AppConfig.ValidDomain
		if !strings.HasSuffix(email, "@"+validDomain) {
			s.getCtxLogger(r).Warnw("found valid session with email outside valid domain",
				"valid_domain", validDomain,
			)
			s.errorMessage(
				http.StatusUnauthorized,
				"Oh dear, it looks like you don't have an @"+validDomain+" account.",
				w, r,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) FowardAuth() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		s.getCtxLogger(r).Infow("forward auth passed",
			"X-Forwarded-Uri", r.Header.Get("X-Forwarded-Uri"),
		)
		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

func (s *Server) OAuth2LoginLink() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		session, err := s.sessions.New(r, "session")
		if err != nil {
			s.getCtxLogger(r).Errorw("got invalid session on login",
				"err", err,
			)
			http.SetCookie(w, emptySessionCookie)
			http.Redirect(w, r, config.AppConfig.BaseURL+"api/oauth2login", http.StatusTemporaryRedirect)
			return nil
		}

		state := uniuri.NewLen(stateLength)
		session.Values["state"] = state

		var url string
		if config.AppConfig.OAuth2UsePKCE {
			codeVerifier := oauth2.GenerateVerifier()
			session.Values["code_verifier"] = codeVerifier

			url = s.oauthConfig.AuthCodeURL(state,
				oauth2.AccessTypeOnline,
				oauth2.S256ChallengeOption(codeVerifier),
			)
		} else {
			url = s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
		}

		s.sessions.Save(r, w, session)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return nil
	}
}

func (s *Server) OAuth2Callback() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		l := s.getCtxLogger(r)
		code := r.FormValue("code")
		state := r.FormValue("state")

		session, err := s.sessions.Get(r, "session")
		if err != nil {
			l.Errorw("got invalid session on login", "err", err)
			http.SetCookie(w, emptySessionCookie)
			http.Redirect(w, r, config.AppConfig.BaseURL+"api/oauth2login", http.StatusTemporaryRedirect)
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

		var token *oauth2.Token
		var tokenErr error

		if config.AppConfig.OAuth2UsePKCE {
			codeVerifier, ok := session.Values["code_verifier"].(string)
			if !ok {
				l.Errorw("failed to get OAuth2 code verifier from session")
				return StatusError{
					http.StatusUnauthorized,
					"Missing PKCE code verifier.",
				}
			}

			token, tokenErr = s.oauthConfig.Exchange(
				r.Context(),
				code,
				oauth2.VerifierOption(codeVerifier),
			)
		} else {
			token, tokenErr = s.oauthConfig.Exchange(r.Context(), code)
		}

		if tokenErr != nil {
			l.Errorw("failed to exchange token", "err", tokenErr)
			return tokenErr
		}

		client := s.oauthConfig.Client(r.Context(), token)
		rawInfo, err := client.Get(s.oidcProvider.UserInfoEndpoint())
		if err != nil {
			l.Errorw("failed to get user info", "err", err)
			return err
		}

		var info struct {
			Email     string   `json:"email"`
			Name      string   `json:"name"`
			GivenName string   `json:"given_name"`
			Groups    []string `json:"groups"`
		}
		if err := json.NewDecoder(rawInfo.Body).Decode(&info); err != nil {
			l.Errorw("failed to decode user info", "err", err)
			return err
		}

		session.Values["email"] = info.Email
		session.Values["name"] = info.Name
		session.Values["first_name"] = info.GivenName
		session.Values["groups"] = info.Groups

		// Clean up OAuth session values
		delete(session.Values, "code_verifier")
		delete(session.Values, "state")

		s.sessions.Save(r, w, session)

		l.Infow("processed login",
			"email", info.Email,
			"name", info.Name,
			"groups", info.Groups,
		)
		http.Redirect(w, r, config.AppConfig.BaseURL, http.StatusTemporaryRedirect)
		return nil
	}
}

func (s *Server) Logout() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		s.getCtxLogger(r).Info("logged out")

		http.SetCookie(w, emptySessionCookie)
		http.Redirect(w, r, config.AppConfig.BaseURL, http.StatusTemporaryRedirect)
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
			s.getCtxLogger(r).Errorw("failed to get site admin status",
				"err", err,
			)
			return err
		}

		courses, err := gi.GetAdminCourses(r.Context(), email)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to get admin courses",
				"err", err,
			)
			return err
		}

		// If any read or assertion fails, the string will be empty and
		// will get caught by omitempty in the JSON encoding. It looks bad,
		// but is actually not horrible!
		name, _ := r.Context().Value(nameContextKey).(string)
		firstName, _ := r.Context().Value(firstNameContextKey).(string)
		groups, _ := r.Context().Value(GroupsContextKey).([]string)

		resp := struct {
			Email        string   `json:"email"`
			SiteAdmin    bool     `json:"site_admin"`
			AdminCourses []string `json:"admin_courses"`
			Name         string   `json:"name"`
			FirstName    string   `json:"first_name"`
			Groups       []string `json:"groups"`
		}{email, admin, courses, name, firstName, groups}

		return s.sendResponse(http.StatusOK, resp, w, r)
	}
}
