/*
Package authtest provides some simple JWT token testing functionality for use in Ensign
SDK tests. This package is disconnected from the Quarterdeck server, so the tests must
be kept up to date with expected Quarterdeck responses.
*/
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/oklog/ulid/v2"
)

const (
	Audience        = "http://127.0.0.1"
	RefreshAudience = "http://127.0.0.1/refresh"
	Issuer          = "http://127.0.0.1"
	AccessDuration  = 10 * time.Minute
	RefreshDuration = 20 * time.Minute
	RefreshOverlap  = -10 * time.Minute
)

var (
	signingMethod = jwt.SigningMethodRS256
)

// Server implements an endpoint to host JWKS public keys and also provides simple
// functionality to create access and refresh tokens that would be authenticated.
type Server struct {
	srv   *httptest.Server
	mux   *http.ServeMux
	url   *url.URL
	key   *rsa.PrivateKey
	keyID ulid.ULID
	authn map[string]string
}

// NewServer starts and returns a new authtest server. The caller should call Close
// when finished, to shut it down.
func NewServer() (s *Server, err error) {
	// Setup routes for the mux
	s = &Server{
		authn: make(map[string]string),
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/v1/status", s.Status)
	s.mux.HandleFunc("/v1/authenticate", s.Authenticate)
	s.mux.HandleFunc("/v1/refresh", s.Refresh)

	// Setup httptest Server
	s.srv = httptest.NewServer(s.mux)
	s.url, _ = url.Parse(s.srv.URL)

	// Create fake keys to create tokens with
	s.keyID = ulid.Make()
	if s.key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Close() {
	s.srv.Close()
}

func (s *Server) URL() string {
	return s.url.String()
}

func (s *Server) ResolveReference(u *url.URL) string {
	return s.url.ResolveReference(u).String()
}

// Register creates a clientID and clientSecret that can be used for authentication.
func (s *Server) Register() (clientID, clientSecret string) {
	cidbuf := make([]byte, 9)
	rand.Read(cidbuf)
	clientID = base64.RawURLEncoding.EncodeToString(cidbuf)

	csbuf := make([]byte, 21)
	rand.Read(csbuf)
	clientSecret = base64.RawURLEncoding.EncodeToString(csbuf)

	s.authn[clientID] = clientSecret
	return clientID, clientSecret
}

func (s *Server) Authenticate(w http.ResponseWriter, r *http.Request) {
	// Deserialize request
	var creds map[string]string
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		Err(w, http.StatusInternalServerError, err)
		return
	}

	// Check credentials
	secret, ok := s.authn[creds["client_id"]]
	if !ok || secret != creds["client_secret"] {
		Err(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	// Create response
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: creds["client_id"],
		},
	}

	atks, rtks, err := s.CreateTokenPair(claims)
	if err != nil {
		Err(w, http.StatusInternalServerError, err)
	}

	rep := map[string]string{
		"access_token":  atks,
		"refresh_token": rtks,
		"last_login":    "todo",
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rep)
}

func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	// Deserialize request
	var creds map[string]string
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		Err(w, http.StatusInternalServerError, err)
		return
	}

	// Validate refresh
	var (
		err   error
		token *jwt.Token
	)

	claims := &Claims{}
	if token, err = jwt.ParseWithClaims(creds["refresh_token"], claims, s.keyFunc); err != nil {
		Err(w, http.StatusInternalServerError, err)
		return
	}

	if !token.Valid {
		Err(w, http.StatusUnauthorized, errors.New("invalid refresh token"))
	}

	// Create response
	atks, rtks, err := s.CreateTokenPair(claims)
	if err != nil {
		Err(w, http.StatusInternalServerError, err)
	}

	rep := map[string]string{
		"access_token":  atks,
		"refresh_token": rtks,
		"last_login":    "todo",
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rep)
}

func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"status":  "ok",
		"uptime":  "todo",
		"version": "test",
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

func (s *Server) CreateTokenPair(claims *Claims) (atks, rtks string, err error) {
	atk := s.CreateAccessToken(claims)
	rtk := s.CreateRefreshToken(atk)

	if atks, err = s.Sign(atk); err != nil {
		return "", "", err
	}

	if rtks, err = s.Sign(rtk); err != nil {
		return "", "", err
	}

	return atks, rtks, nil
}

func (s *Server) CreateAccessToken(claims *Claims) *jwt.Token {
	now := time.Now()
	sub := claims.RegisteredClaims.Subject
	id := strings.ToLower(ulid.Make().String())

	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        id,
		Subject:   sub,
		Audience:  jwt.ClaimStrings{Audience},
		Issuer:    Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(AccessDuration)),
	}

	return s.CreateToken(claims)
}

func (s *Server) CreateRefreshToken(accessToken *jwt.Token) *jwt.Token {
	accessClaims := accessToken.Claims.(*Claims)
	audience := append(accessClaims.Audience, RefreshAudience)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessClaims.ID, // ID is randomly generated and shared between access and refresh tokens.
			Audience:  audience,
			Issuer:    accessClaims.Issuer,
			Subject:   accessClaims.Subject,
			IssuedAt:  accessClaims.IssuedAt,
			NotBefore: jwt.NewNumericDate(accessClaims.ExpiresAt.Add(RefreshOverlap)),
			ExpiresAt: jwt.NewNumericDate(accessClaims.IssuedAt.Add(RefreshDuration)),
		},
	}
	return s.CreateToken(claims)
}

func (s *Server) CreateToken(claims *Claims) *jwt.Token {
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{Audience}
	}

	if claims.Issuer == "" {
		claims.Issuer = Issuer
	}
	return jwt.NewWithClaims(signingMethod, claims)
}

func (s *Server) Sign(token *jwt.Token) (tks string, err error) {
	token.Header["kid"] = s.keyID.String()
	return token.SignedString(s.key)
}

func (s *Server) keyFunc(token *jwt.Token) (key interface{}, err error) {
	return &s.key.PublicKey, nil
}

func Err(w http.ResponseWriter, status int, err error) {
	rep := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(rep)
}
