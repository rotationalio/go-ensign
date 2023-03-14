package sdk_test

import (
	"os"
	"testing"

	sdk "github.com/rotationalio/go-ensign"
	"github.com/stretchr/testify/require"
)

func TestNilWilNilOpts(t *testing.T) {
	os.Setenv("ENSIGN_NO_AUTHENTICATION", "true")
	_, err := sdk.New(nil)
	require.NoError(t, err, "could not pass nil into ensign")
}

func TestOptions(t *testing.T) {
	opts := &sdk.Options{
		Endpoint:     "localhost:443",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	// Test ClientID is required
	opts.Endpoint = "localhost:443"
	opts.ClientID = ""
	require.EqualError(t, opts.Validate(), sdk.ErrMissingClientID.Error(), "opts should be invalid with missing client ID")

	// Test ClientSecret is required
	opts.ClientID = "client-id"
	opts.ClientSecret = ""
	require.EqualError(t, opts.Validate(), sdk.ErrMissingClientSecret.Error(), "opts should be invalid with missing client secret")

	// Test valid options
	opts.ClientSecret = "client-secret"
	require.NoError(t, opts.Validate(), "opts should be valid")
}
