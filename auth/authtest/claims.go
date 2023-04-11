package authtest

import "github.com/golang-jwt/jwt/v4"

// Claims implements Quarterdeck-like claims for use in testing the SDK client.
type Claims struct {
	jwt.RegisteredClaims
	OrgID       string   `json:"org,omitempty"`
	ProjectID   string   `json:"project,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}
