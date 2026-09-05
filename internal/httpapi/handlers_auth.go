package httpapi

import (
	"log"
	"net/http"
	"time"

	"github.com/gatiella/deriv-signal-bot/internal/authflow"
	"github.com/gatiella/deriv-signal-bot/internal/deriv"
)

const sessionCookieName = "dsb_session"

// handleLogin redirects the browser to Deriv's login page. After the user
// logs in there (on Deriv's own site - we never see their password),
// Deriv redirects back to /auth/deriv/callback.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	url := authflow.LoginURL(s.cfg.OAuthLoginURL, s.cfg.AppID)
	http.Redirect(w, r, url, http.StatusFound)
}

// handleCallback receives the acctN/tokenN/curN params Deriv appends to our
// redirect URL, authorizes each account to find out who it belongs to, and
// creates a local user + session.
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	accounts := authflow.ParseCallback(r.URL.Query())
	if len(accounts) == 0 {
		http.Error(w, "no accounts returned by Deriv - login may have been cancelled", http.StatusBadRequest)
		return
	}

	// Authorize the first account to establish identity (email is the same
	// across all of a user's accounts).
	primary := accounts[0]
	wsURL := s.cfg.DerivWSURL + "?app_id=" + s.cfg.AppID
	client, err := deriv.Connect(wsURL)
	if err != nil {
		log.Printf("callback: connect failed: %v", err)
		http.Error(w, "could not reach Deriv, try again", http.StatusBadGateway)
		return
	}
	defer client.Close()

	authResult, err := client.Authorize(primary.Token)
	if err != nil {
		log.Printf("callback: authorize failed: %v", err)
		http.Error(w, "Deriv rejected that login, try again", http.StatusBadGateway)
		return
	}

	user, err := s.store.FindOrCreateUserByEmail(authResult.Email)
	if err != nil {
		log.Printf("callback: find/create user failed: %v", err)
		http.Error(w, "internal error creating your account", http.StatusInternalServerError)
		return
	}

	// Store every returned account (real + demo), each with its own encrypted token.
	for _, acct := range accounts {
		encToken, err := s.crypto.Encrypt(acct.Token)
		if err != nil {
			log.Printf("callback: encrypt token failed: %v", err)
			continue
		}
		if err := s.store.UpsertDerivAccount(user.ID, acct.LoginID, acct.Currency, authflow.IsVirtual(acct.LoginID), encToken); err != nil {
			log.Printf("callback: store account %s failed: %v", acct.LoginID, err)
		}
	}

	sessionID, err := s.store.CreateSession(user.ID)
	if err != nil {
		log.Printf("callback: create session failed: %v", err)
		http.Error(w, "internal error starting your session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusOK)
}

// currentUserID resolves the session cookie on the request, if any.
func (s *Server) currentUserID(r *http.Request) (int64, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, false
	}
	return s.store.UserIDForSession(c.Value)
}
