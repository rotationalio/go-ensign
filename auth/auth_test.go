package auth_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/auth/authtest"
	"github.com/stretchr/testify/suite"
)

type authTestSuite struct {
	suite.Suite
	srv  *authtest.Server
	auth *auth.Client
}

func (s *authTestSuite) SetupSuite() {
	var err error
	assert := s.Assert()

	s.srv, err = authtest.NewServer()
	assert.NoError(err, "could not create authtest server")

	s.auth, err = auth.New(s.srv.URL(), false)
	assert.NoError(err, "could not create auth client")
}

func (s *authTestSuite) TearDownSuite() {
	s.srv.Close()
}

func TestAuth(t *testing.T) {
	suite.Run(t, &authTestSuite{})
}

func (s *authTestSuite) TestLogin() {
	require := s.Require()
	clientID, clientSecret := s.srv.Register()

	creds, err := s.auth.Login(context.Background(), clientID, clientSecret)
	require.NoError(err, "could not login with credentials")
	require.NotZero(creds, "expected credentials to be returned")

	// Credentials should be cached if valid so the same creds should be returned
	other, err := s.auth.Credentials(context.Background())
	require.NoError(err, "could not fetch credentials")

	credsc, ok := creds.(*auth.Credentials)
	require.True(ok, "could not convert creds to credentials")
	otherc, ok := other.(*auth.Credentials)
	require.True(ok, "could not convert other creds  to credentials")
	require.True(credsc.Equals(otherc))
}

func (s *authTestSuite) TestLoginError() {
	require := s.Require()
	ctx := context.Background()

	// Cannot login without credentials
	_, err := s.auth.Login(ctx, "", "")
	require.ErrorIs(err, auth.ErrIncompleteCreds)
	_, err = s.auth.Login(ctx, "foo", "")
	require.ErrorIs(err, auth.ErrIncompleteCreds)
	_, err = s.auth.Login(ctx, "", "foo")
	require.ErrorIs(err, auth.ErrIncompleteCreds)

	// Cannot login with incorrect credentials
	_, err = s.auth.Login(ctx, "hacker", "password")
	require.EqualError(err, "[401] invalid credentials")
}

func (s *authTestSuite) TestAuthenticate() {
	require := s.Require()

	req := &auth.APIKey{}
	req.ClientID, req.ClientSecret = s.srv.Register()

	rep, err := s.auth.Authenticate(context.Background(), req)
	require.NoError(err, "could not authenticate with good credentials")
	require.NotZero(rep, "no response returned")
	require.NotZero(rep.AccessToken, "no access token returned")
	require.NotZero(rep.RefreshToken, "no refresh token returned")

	// Test authenticate requires apikeys
	_, err = s.auth.Authenticate(context.Background(), nil)
	require.ErrorIs(err, auth.ErrNoAPIKeys)
	_, err = s.auth.Authenticate(context.Background(), &auth.APIKey{})
	require.ErrorIs(err, auth.ErrNoAPIKeys)
	_, err = s.auth.Authenticate(context.Background(), &auth.APIKey{ClientID: "foo"})
	require.ErrorIs(err, auth.ErrNoAPIKeys)
	_, err = s.auth.Authenticate(context.Background(), &auth.APIKey{ClientSecret: "foo"})
	require.ErrorIs(err, auth.ErrNoAPIKeys)
}

func (s *authTestSuite) TestRefresh() {
	var err error
	require := s.Require()

	// Test valid refresh
	req := &auth.Tokens{}
	claims := &authtest.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "testing"}}
	req.AccessToken, req.RefreshToken, err = s.srv.CreateTokenPair(claims)
	require.NoError(err, "could not create tokens to test refresh")

	rep, err := s.auth.Refresh(context.Background(), req)
	require.NoError(err, "could not refresh tokens")
	require.NotZero(rep, "no access tokens returned")
	require.NotEqual(rep.AccessToken, req.AccessToken)
	require.NotEqual(rep.RefreshToken, req.RefreshToken)

	// Test invalid refresh
	req.RefreshToken = "bad refresh token"
	_, err = s.auth.Refresh(context.Background(), req)
	require.EqualError(err, "[500] token contains an invalid number of segments")

	// Test expired token
	claims = &authtest.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)),
		},
	}
	req.RefreshToken, err = s.srv.Sign(s.srv.CreateToken(claims))
	require.NoError(err, "could not create expired refresh token")

	_, err = s.auth.Refresh(context.Background(), req)
	require.Error(err, "expected an error returned")
	require.Regexp(regexp.MustCompile(`^\[500\] token is expired by 5m0\.(\d+)s$`), err.Error())
}

func (s *authTestSuite) TestStatus() {
	require := s.Require()
	status, err := s.auth.Status(context.Background())
	require.NoError(err, "could not make status request")
	require.Equal("ok", status.Status)
	require.Equal("test", status.Version)
}
