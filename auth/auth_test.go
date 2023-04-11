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

func (s *authTestSuite) AfterTest(suiteName, testName string) {
	s.auth.Reset()
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

func (s *authTestSuite) TestCredentials() {
	require := s.Require()
	ctx := context.Background()

	// If tokens and apikeys are nil, an error should be returned
	s.auth.Reset()
	_, err := s.auth.Credentials(ctx)
	require.ErrorIs(err, auth.ErrNoAPIKeys)

	// If apikeys are set, then the tokens should be correctly authenticated
	apikey := &auth.APIKey{}
	apikey.ClientID, apikey.ClientSecret = s.srv.Register()
	s.auth.SetAPIKey(apikey)

	creds, err := s.auth.Credentials(ctx)
	require.NoError(err, "could not authenticate to test server")

	// Creds should be cached while they are still valid
	other, err := s.auth.Credentials(ctx)
	require.NoError(err, "could not authenticate to test server")
	require.Equal(creds, other, "expected creds to be identical since they are cached")

	// If the tokens are not valid an error should be returned
	s.auth.SetTokens(&auth.Tokens{AccessToken: "invalid", RefreshToken: "invalid"})
	_, err = s.auth.Credentials(ctx)
	require.EqualError(err, "token contains an invalid number of segments")

	// If the access token is expired but the refresh token is valid, should refresh
	unexpired := &authtest.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	expired := &authtest.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)),
		},
	}

	tokens := &auth.Tokens{}
	tokens.AccessToken, err = s.srv.Sign(s.srv.CreateToken(expired))
	require.NoError(err, "could not create expired access token")
	tokens.RefreshToken, err = s.srv.Sign(s.srv.CreateToken(unexpired))
	require.NoError(err, "could not create unexpired refresh token")
	s.auth.SetTokens(tokens)

	other, err = s.auth.Credentials(ctx)
	require.NoError(err, "could not authenticate to test server")
	require.NotEqual(creds, other, "expected new creds to be issued from refresh")

	// Should reauthenticate if both the access token and the refresh token are expired
	// NOTE: must create new tokens struct to avoid cached timestamps
	tokens = &auth.Tokens{}
	tokens.AccessToken, err = s.srv.Sign(s.srv.CreateToken(expired))
	require.NoError(err, "could not create expired access token")
	tokens.RefreshToken, err = s.srv.Sign(s.srv.CreateToken(expired))
	require.NoError(err, "could not create expired refresh token")
	s.auth.SetTokens(tokens)

	compr, err := s.auth.Credentials(ctx)
	require.NoError(err, "could not authenticate to test server")
	require.NotEqual(other, compr, "expected new creds to be issued from authenticate")

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
