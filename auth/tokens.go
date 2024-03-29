package auth

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

var parser = &jwt.Parser{SkipClaimsValidation: true}

func Parse(tks string) (claims *jwt.RegisteredClaims, err error) {
	claims = &jwt.RegisteredClaims{}
	if _, _, err = parser.ParseUnverified(tks, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func ExpiresAt(tks string) (_ time.Time, err error) {
	var claims *jwt.RegisteredClaims
	if claims, err = Parse(tks); err != nil {
		return time.Time{}, err
	}
	return claims.ExpiresAt.Time, nil
}

func NotBefore(tks string) (_ time.Time, err error) {
	var claims *jwt.RegisteredClaims
	if claims, err = Parse(tks); err != nil {
		return time.Time{}, err
	}
	return claims.NotBefore.Time, nil
}
