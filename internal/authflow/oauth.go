// Package authflow implements Deriv's "Login with Deriv" OAuth redirect,
// the same mechanism DTrader-style third-party apps use.
//
// How it works (see https://developers.deriv.com/docs/intro/oauth for the
// newer PKCE variant; this implements the simpler classic redirect flow used
// by apps registered against the classic ws.derivws.com/websockets/v3 API,
// which is what this whole project talks to):
//
//  1. You register an application at https://app.deriv.com/account/api-token
//     (or the dashboard linked from developers.deriv.com) and set its
//     "Redirect URL" to point at YOUR_DOMAIN/auth/deriv/callback.
//  2. You send the user to LoginURL(appID).
//  3. Deriv shows its own login screen - the user logs into THEIR Deriv
//     account, your app never sees their Deriv password.
//  4. Deriv redirects back to your redirect URL with one or more accounts
//     in the query string, e.g.:
//     ?acct1=CR123456&token1=abc123&cur1=USD&acct2=VRTC98765&token2=def456&cur2=USD
//     (acct1/token1 is typically their real account, acct2/token2 a demo
//     account, if they have both).
//  5. ParseCallback extracts every (loginid, token, currency) triple so you
//     can store them.
package authflow

import (
	"fmt"
	"net/url"
	"strings"
)

// LoginURL builds the URL to send the user's browser to in order to start
// the Deriv login flow.
func LoginURL(baseLoginURL, appID string) string {
	v := url.Values{}
	v.Set("app_id", appID)
	v.Set("l", "EN")
	return fmt.Sprintf("%s?%s", baseLoginURL, v.Encode())
}

// CallbackAccount is one Deriv account returned in the OAuth callback.
type CallbackAccount struct {
	LoginID  string
	Token    string
	Currency string
}

// ParseCallback extracts every acctN/tokenN/curN triple from the callback
// query parameters. Deriv numbers accounts starting at 1 and stops at the
// first missing index.
func ParseCallback(query url.Values) []CallbackAccount {
	var accounts []CallbackAccount
	for i := 1; ; i++ {
		acct := query.Get(fmt.Sprintf("acct%d", i))
		token := query.Get(fmt.Sprintf("token%d", i))
		if acct == "" || token == "" {
			break
		}
		cur := query.Get(fmt.Sprintf("cur%d", i))
		accounts = append(accounts, CallbackAccount{
			LoginID:  acct,
			Token:    token,
			Currency: cur,
		})
	}
	return accounts
}

// IsVirtual reports whether a Deriv login id belongs to a demo/virtual
// account. Deriv's convention is a "VRTC" (or "VRW") prefix for virtual
// accounts and "CR"/"MF"/etc for real ones.
func IsVirtual(loginID string) bool {
	return strings.HasPrefix(loginID, "VRT")
}
