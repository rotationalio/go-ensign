package auth_test

import (
	"testing"

	"github.com/rotationalio/go-ensign/auth"
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
