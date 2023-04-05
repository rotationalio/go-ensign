package auth_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rotationalio/go-ensign/auth"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tokens, err := loadTokensFixture("testdata/tokens.json")
	require.NoError(t, err, "could not load tokens fixture")

	accessClaims, err := auth.Parse(tokens.AccessToken)
	require.NoError(t, err, "could not parse access token")

	refreshClaims, err := auth.Parse(tokens.RefreshToken)
	require.NoError(t, err, "could not parse refresh token")

	// We expect the claims and refresh tokens to have the same ID
	require.Equal(t, accessClaims.ID, refreshClaims.ID, "access and refresh token had different IDs or the parse was unsuccessful")

	// Check that an error is returned when parsing a bad token
	_, err = auth.Parse("notarealtoken")
	require.Error(t, err, "should not be able to parse a bad token")
}

func TestExpiresAt(t *testing.T) {
	tokens, err := loadTokensFixture("testdata/tokens.json")
	require.NoError(t, err, "could not load tokens fixture")

	expiration, err := auth.ExpiresAt(tokens.AccessToken)
	require.NoError(t, err, "could not parse access token")

	// Expect the time to be fetched correctly from the token
	expected := time.Date(2023, 4, 4, 13, 35, 30, 0, time.UTC)
	require.True(t, expected.Equal(expiration))

	// Check that an error is returned when parsing a bad token
	_, err = auth.ExpiresAt("notarealtoken")
	require.Error(t, err, "should not be able to parse a bad token")
}

func TestNotBefore(t *testing.T) {
	tokens, err := loadTokensFixture("testdata/tokens.json")
	require.NoError(t, err, "could not load tokens fixture")

	expiration, err := auth.NotBefore(tokens.RefreshToken)
	require.NoError(t, err, "could not parse access token")

	// Expect the time to be fetched correctly from the token
	expected := time.Date(2023, 4, 4, 13, 20, 30, 0, time.UTC)
	require.True(t, expected.Equal(expiration))

	// Check that an error is returned when parsing a bad token
	_, err = auth.NotBefore("notarealtoken")
	require.Error(t, err, "should not be able to parse a bad token")
}

func loadTokensFixture(path string) (tokens *auth.Tokens, err error) {
	var f *os.File
	if f, err = os.Open(path); err != nil {
		return nil, err
	}
	defer f.Close()

	tokens = &auth.Tokens{}
	if err = json.NewDecoder(f).Decode(tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}
