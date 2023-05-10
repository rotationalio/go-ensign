package ensign_test

import (
	"os"
	"testing"

	sdk "github.com/rotationalio/go-ensign"
	"github.com/rotationalio/go-ensign/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var testEnv = map[string]string{
	"ENSIGN_ENDPOINT":          "ensign.ninja:443",
	"ENSIGN_CLIENT_ID":         "testing123",
	"ENSIGN_CLIENT_SECRET":     "supersecretsquirrel",
	"ENSIGN_INSECURE":          "true",
	"ENSIGN_AUTH_URL":          "https://auth.ensign.world",
	"ENSIGN_NO_AUTHENTICATION": "1",
}

func TestNewOptionsDefaults(t *testing.T) {
	opts, err := sdk.NewOptions(sdk.WithCredentials("testing123", "supersecret"))
	require.NoError(t, err, "could not create opts with credentials")

	require.Equal(t, "testing123", opts.ClientID)
	require.Equal(t, "supersecret", opts.ClientSecret)
	require.Equal(t, sdk.EnsignEndpoint, opts.Endpoint)
	require.Equal(t, sdk.AuthEndpoint, opts.AuthURL)
	require.False(t, opts.Insecure)
	require.False(t, opts.NoAuthentication)
}

func TestNewOptionsEnv(t *testing.T) {
	// Set required environment variables and cleanup after the test is complete.
	t.Cleanup(cleanupEnv())
	setEnv()

	opts, err := sdk.NewOptions()
	require.NoError(t, err, "expected options fully set from environment")

	require.Equal(t, testEnv[sdk.EnvEndpoint], opts.Endpoint, "expected endpoint set from env")
	require.Equal(t, testEnv[sdk.EnvClientID], opts.ClientID, "expected client id set from env")
	require.Equal(t, testEnv[sdk.EnvClientSecret], opts.ClientSecret, "expected client secret set from env")
	require.True(t, opts.Insecure, "expected insecure set to true from env")
	require.Equal(t, testEnv[sdk.EnvAuthURL], opts.AuthURL, "expected auth url set from env")
	require.True(t, opts.NoAuthentication, "expected no authentication set to true from env")
}

func TestLoadCredentials(t *testing.T) {
	opts, err := sdk.NewOptions(sdk.WithLoadCredentials("testdata/client.json"))
	require.NoError(t, err, "could not load credentials from file")

	require.Equal(t, "ABCDEFgHijKLMnopQRsTuvWXyZABCDEf", opts.ClientID)
	require.Equal(t, "a12BcD3EF45gHI6jKLmnOpQ78RStUVWXYzabCdE9FGHijkLmNOpq0RStUvwxyzab", opts.ClientSecret)

	_, err = sdk.NewOptions(sdk.WithLoadCredentials("testdata/doesnotexist.json"))
	require.Error(t, err, "should have been an error returned if data couldn't be loaded")
}

func TestWithEnsignEndpoint(t *testing.T) {
	opts, err := sdk.NewOptions(
		sdk.WithCredentials("testing123", "supersecret"),
		sdk.WithEnsignEndpoint("events.ensign.world:443", true, grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	require.NoError(t, err, "could not create opts with ensign endpoint")

	require.Equal(t, "events.ensign.world:443", opts.Endpoint)
	require.True(t, opts.Insecure)
	require.Len(t, opts.Dialing, 1)
}

func TestWithAuthenticator(t *testing.T) {
	opts, err := sdk.NewOptions(
		sdk.WithCredentials("testing123", "supersecret"),
		sdk.WithAuthenticator("https://auth.ensign.ninja", true),
	)
	require.NoError(t, err, "could not create opts with authenticator")

	require.Equal(t, "https://auth.ensign.ninja", opts.AuthURL)
	require.True(t, opts.NoAuthentication)
}

func TestWithOptions(t *testing.T) {
	original := sdk.Options{
		ClientID:     "originalID",
		ClientSecret: "originalSecret",
		Endpoint:     "original:443",
		AuthURL:      "https://original.com",
	}

	opts, err := sdk.NewOptions(sdk.WithOptions(original))
	require.NoError(t, err, "could not create opts with options struct")

	require.NotSame(t, original, opts, "original and opts should not be the same object")
	require.Equal(t, original, opts, "original and opts should be identical")
}

func TestWithMock(t *testing.T) {
	mock := mock.New(nil)
	opts, err := sdk.NewOptions(sdk.WithMock(mock, grpc.WithTransportCredentials(insecure.NewCredentials())))
	require.NoError(t, err, "could not create opts mock")

	require.True(t, opts.Testing, "expected opts to be in testing mode")
	require.Same(t, opts.Mock, mock, "expected mock to be set on opts")
	require.Len(t, opts.Dialing, 1, "expected one dial option to be set on mock")
}

func TestOptionsDefaults(t *testing.T) {
	opts := &sdk.Options{
		ClientID:     "testing123",
		ClientSecret: "supersecretsquirrel",
	}

	err := opts.Validate()
	require.NoError(t, err, "should be able to validate opts with just client id and secret")
	require.Equal(t, sdk.EnsignEndpoint, opts.Endpoint)
	require.Equal(t, sdk.AuthEndpoint, opts.AuthURL)
	require.False(t, opts.Insecure)
	require.False(t, opts.NoAuthentication)
	require.False(t, opts.Testing)
	require.Nil(t, opts.Mock)
	require.Nil(t, opts.Dialing)
}

func TestOptionsSetFromEnvironment(t *testing.T) {
	// Set required environment variables and cleanup after the test is complete.
	t.Cleanup(cleanupEnv())
	setEnv()

	opts := &sdk.Options{}
	err := opts.Validate()
	require.NoError(t, err, "expected options fully set from environment")

	require.Equal(t, testEnv[sdk.EnvEndpoint], opts.Endpoint, "expected endpoint set from env")
	require.Equal(t, testEnv[sdk.EnvClientID], opts.ClientID, "expected client id set from env")
	require.Equal(t, testEnv[sdk.EnvClientSecret], opts.ClientSecret, "expected client secret set from env")
	require.True(t, opts.Insecure, "expected insecure set to true from env")
	require.Equal(t, testEnv[sdk.EnvAuthURL], opts.AuthURL, "expected auth url set from env")
	require.True(t, opts.NoAuthentication, "expected no authentication set to true from env")
	require.False(t, opts.Testing, "expected testing to be false")
	require.Nil(t, opts.Mock, "expected mock to be nil")
	require.Nil(t, opts.Dialing, "expected dialing options to be nil")
}

func TestSetCredentialsPriority(t *testing.T) {
	// If credentials are set on the options they shouldn't be overridden by the env.
	// Set required environment variables and cleanup after the test is complete.
	t.Cleanup(cleanupEnv())
	setEnv()

	opts := &sdk.Options{
		ClientID:     "original",
		ClientSecret: "originalsecret",
	}
	err := opts.Validate()
	require.NoError(t, err, "expected valid options")

	require.Equal(t, "original", opts.ClientID)
	require.Equal(t, "originalsecret", opts.ClientSecret)
}

func TestOptionsValidation(t *testing.T) {
	opts := &sdk.Options{
		ClientID:         "testing123",
		ClientSecret:     "supersecret",
		Endpoint:         "ensign.world:443",
		Dialing:          nil,
		AuthURL:          "https://auth.ensign.world",
		Insecure:         false,
		NoAuthentication: false,
		Testing:          false,
		Mock:             nil,
	}

	// Complete credentials should be valid
	err := opts.Validate()
	require.NoError(t, err, "expected complete credentials to be valid")

	// ClientID required
	opts.ClientID = ""
	err = opts.Validate()
	require.ErrorIs(t, err, sdk.ErrMissingClientID, "client id should be required")
	opts.ClientID = "testing123"

	// ClientSecret required
	opts.ClientSecret = ""
	err = opts.Validate()
	require.ErrorIs(t, err, sdk.ErrMissingClientSecret, "client secret should be required")
	opts.ClientSecret = "supersecret"

	// NOTE: cannot validate Endpoint and AuthURL required since the defaults will be set.
}

func TestCredsNotRequired(t *testing.T) {
	// Credentials should not be required if NoAuthentication is true
	opts := &sdk.Options{NoAuthentication: true}
	err := opts.Validate()
	require.NoError(t, err, "no credentials are required if NoAuthentication is true")
	require.Empty(t, opts.ClientID, "unexepcted value set for test")
	require.Empty(t, opts.ClientSecret, "unexepcted value set for test")
}

func TestTestingOptions(t *testing.T) {
	// Only mock is required in testing mode
	opts := &sdk.Options{Testing: true, Mock: mock.New(nil)}
	err := opts.Validate()
	require.NoError(t, err, "only mock is required in testing mode")

	// Mock required in testing mode
	opts.Mock = nil
	err = opts.Validate()
	require.ErrorIs(t, err, sdk.ErrMissingMock)
}

// Returns the current environment for the specified keys, or if no keys are specified
// then it returns the current environment for all keys in the testEnv variable.
func curEnv(keys ...string) map[string]string {
	env := make(map[string]string)
	if len(keys) > 0 {
		for _, key := range keys {
			if val, ok := os.LookupEnv(key); ok {
				env[key] = val
			}
		}
	} else {
		for key := range testEnv {
			env[key] = os.Getenv(key)
		}
	}

	return env
}

// Sets the environment variables from the testEnv variable. If no keys are specified,
// then this function sets all environment variables from the testEnv.
func setEnv(keys ...string) {
	if len(keys) > 0 {
		for _, key := range keys {
			if val, ok := testEnv[key]; ok {
				os.Setenv(key, val)
			}
		}
	} else {
		for key, val := range testEnv {
			os.Setenv(key, val)
		}
	}
}

// Cleanup helper function that can be run when the tests are complete to reset the
// environment back to its previous state before the test was run.
func cleanupEnv(keys ...string) func() {
	prevEnv := curEnv(keys...)
	return func() {
		for key, val := range prevEnv {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}
}
