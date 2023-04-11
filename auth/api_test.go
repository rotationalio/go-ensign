package auth_test

import (
	"testing"

	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/auth/authtest"
	"github.com/stretchr/testify/require"
)

func TestExpiredTokens(t *testing.T) {
	tokens, err := loadTokensFixture("testdata/tokens.json")
	require.NoError(t, err, "could not load tokens fixture")

	valid, err := tokens.AccessValid()
	require.NoError(t, err, "could not parse access tokens")
	require.False(t, valid, "fixture tokens should be expired")

	valid, err = tokens.RefreshValid()
	require.NoError(t, err, "could not parse refresh tokens")
	require.False(t, valid, "fixture tokens should be expired")
}

func TestBadTokens(t *testing.T) {
	tokens := &auth.Tokens{}

	valid, err := tokens.AccessValid()
	require.Error(t, err, "expected error when access token is missing")
	require.False(t, valid, "expected valid false when access token is missing")

	valid, err = tokens.RefreshValid()
	require.Error(t, err, "expected error when refresh token is missing")
	require.False(t, valid, "expected valid false when refresh token is missing")
}

func (s *authTestSuite) TestValidTokens() {
	require := s.Require()

	atks, rtks, err := s.srv.CreateTokenPair(&authtest.Claims{})
	require.NoError(err, "could not create access and refresh token")

	tokens := &auth.Tokens{
		AccessToken:  atks,
		RefreshToken: rtks,
	}

	valid, err := tokens.AccessValid()
	require.NoError(err, "could not validate access token")
	require.True(valid, "expected access token to be valid")

	valid, err = tokens.RefreshValid()
	require.NoError(err, "could not validate refresg token")
	require.True(valid, "expected refresh token to be valid")
}
