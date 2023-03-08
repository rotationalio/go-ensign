package sdk_test

import (
	"testing"

	sdk "github.com/rotationalio/go-ensign"
	"github.com/stretchr/testify/require"
)

func TestNilWilNilOpts(t *testing.T) {
	_, err := sdk.New(nil)
	require.NoError(t, err, "could not pass nil into ensign")
}

func TestOptions(t *testing.T) {
	opts := &sdk.Options{
		Endpoint:     "localhost:443",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	// Test Endpoint is required
	opts.Endpoint = ""
	require.EqualError(t, opts.Validate(), sdk.ErrMissingEndpoint.Error(), "opts should be invalid with missing endpoint")

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
