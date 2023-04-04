package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Reply contains standard fields that are used for generic API responses and errors.
type Reply struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// Status describes the current state of the Quarterdeck service. This struct is used to
// GET JSON requests from the Quarterdeck service.
type Status struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime,omitempty"`
	Version string `json:"version,omitempty"`
}

// APIKey wraps per-project Ensign credentials and can be stored as JSON on disk. This
// struct is also used to POST JSON requests to the Quarterdeck service.
type APIKey struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Tokens are handed out by Quarterdeck to login to the Ensign service. The AccessToken
// is used to create gRPC per-RPC credentials and the refresh token is used to fetch a
// new access token when it expires. Tokens can be cached as JSON on disk. This struct
// is also used to GET/POST JSON requests from/to the Quarterdeck service.
type Tokens struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	LastLogin    string `json:"last_login,omitempty"`

	// Cached accessors for jwt timestamps parsed from the tokens
	accessExpires    time.Time
	refreshExpires   time.Time
	refreshNotBefore time.Time
}

// AccessValid returns true if the access token has not expired
func (t *Tokens) AccessValid() (valid bool, err error) {
	if t.accessExpires.IsZero() {
		if t.accessExpires, err = ExpiresAt(t.AccessToken); err != nil {
			return false, err
		}
	}
	return time.Now().Before(t.accessExpires), nil
}

// RefreshValid returns true if the refresh token has not expired and it is after the
// not before time when the token cannot yet be used.
func (t *Tokens) RefreshValid() (valid bool, err error) {
	if t.refreshExpires.IsZero() || t.refreshNotBefore.IsZero() {
		// Parse the refresh token
		var claims *jwt.RegisteredClaims
		if claims, err = Parse(t.RefreshToken); err != nil {
			return false, err
		}

		t.refreshExpires = claims.ExpiresAt.Time
		t.refreshNotBefore = claims.NotBefore.Time
	}
	now := time.Now()
	return now.After(t.refreshNotBefore) && now.Before(t.refreshExpires), nil
}
