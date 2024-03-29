package ensign

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rotationalio/go-ensign/mock"
	"google.golang.org/grpc"
)

// Environment variables for configuring Ensign. Unless otherwise specified in the
// options, Ensign will try to configure your client from the environment.
const (
	// APIKeys you receive when creating a project on https://rotational.app will return
	// a Client ID and Client Secret. These should be specified in the environment with
	// the following environment variables. You cannot connect to Ensign without these
	// APIKeys. Note that APIKeys only allow connections to one project in Ensign. In
	// the future, Ensign will be able to support multiple key credentials to access
	// multiple projects.
	EnvClientID     = "ENSIGN_CLIENT_ID"
	EnvClientSecret = "ENSIGN_CLIENT_SECRET"

	// The following environment variables allow you to connect to another Ensign
	// service. These are primarily used by the Rotational team to connect to develop
	// and staging Ensign environments. However, if you have an Enterprise deployment of
	// Ensign, you may need to specify these alternative connection details.
	EnvEndpoint = "ENSIGN_ENDPOINT"
	EnvInsecure = "ENSIGN_INSECURE"
	EnvAuthURL  = "ENSIGN_AUTH_URL"
	EnvNoAuth   = "ENSIGN_NO_AUTHENTICATION"
)

// Default connection endpoints to the production Ensign cluster.
const (
	EnsignEndpoint = "ensign.rotational.app:443"
	AuthEndpoint   = "https://auth.rotational.app"
)

// Option allows users to specify variadic options to create & connect the Ensign client.
type Option func(o *Options) error

// WithCredentials allows you to instantiate an Ensign client with API Key information.
func WithCredentials(clientID, clientSecret string) Option {
	return func(o *Options) error {
		o.ClientID = clientID
		o.ClientSecret = clientSecret
		return nil
	}
}

// Keys for credentials dumped as JSON credentials
const (
	keyClientID     = "ClientID"
	keyClientSecret = "ClientSecret"
)

// WithLoadCredentials loads the Ensign API Key information from the JSON file that was
// download from the Rotational web application. Pass in the path to the credentials on
// disk to load them with this option!
func WithLoadCredentials(path string) Option {
	return func(o *Options) (err error) {
		var f *os.File
		if f, err = os.Open(path); err != nil {
			return err
		}
		defer f.Close()

		data := make(map[string]interface{})
		if err = json.NewDecoder(f).Decode(&data); err != nil {
			return err
		}

		// Fetch and parse clientID
		if val, ok := data[keyClientID]; ok {
			if clientID, ok := val.(string); ok && clientID != "" {
				o.ClientID = clientID
			}
		}

		// Fetch and parse clientSecret
		if val, ok := data[keyClientSecret]; ok {
			if clientSecret, ok := val.(string); ok && clientSecret != "" {
				o.ClientSecret = clientSecret
			}
		}

		return nil
	}
}

// WithEnsignEndpoint allows you to specify an endpoint that is not the production
// Ensign cloud. This is useful if you're running an Ensign node in CI or connecting to
// a mock in local tests. Ensign developers may also use this to connect to staging.
// If any gRPC dial options are specified, they override the default Ensign dial options
// including the interceptors that perform authentication -- use only if you know what
// you're doing and why!
func WithEnsignEndpoint(endpoint string, insecure bool, opts ...grpc.DialOption) Option {
	return func(o *Options) error {
		o.Endpoint = endpoint
		o.Dialing = opts
		o.Insecure = insecure
		return nil
	}
}

// WithAuthenticator specifies a different Quarterdeck URL or you can supply an empty
// string and noauth set to true to have no authentication occur with the Ensign client.
func WithAuthenticator(url string, noauth bool) Option {
	return func(o *Options) error {
		o.AuthURL = url
		o.NoAuthentication = noauth
		return nil
	}
}

// WithOptions sets the options to the passed in options value. Note that this will
// override everything in the processing chain including zero-valued items; so use this
// as the first variadic option in NewOptions to guarantee correct processing.
func WithOptions(opts Options) Option {
	return func(o *Options) error {
		*o = opts
		return nil
	}
}

// WithMock connects ensign to the specified mock ensign server for local testing.
func WithMock(mock *mock.Ensign, opts ...grpc.DialOption) Option {
	return func(o *Options) error {
		o.Testing = true
		o.Mock = mock
		o.Dialing = opts
		return nil
	}
}

// Options specifies the client configuration for authenticating and connecting to
// the Ensign service. The goal of the options struct is to be as minimal as possible.
// If users set their credentials via the environment, they should not have to specify
// any options at all to connect. The options does give the client flexibility to
// connect to Ensign nodes in other environments and is primarily for advanced usage.
type Options struct {
	// The API Key credentials include the client ID and secret, both of which are
	// required to authenticate with Ensign via the authentication service so that an
	// access token can be retrieved and placed in all Ensign requests. The only time
	// these settings are not required is if NoAuthentication is true.
	ClientID     string
	ClientSecret string

	// The gRPC endpoint of the Ensign service; by default the EnsignEndpoint.
	Endpoint string

	// Dial options allows the user to specify gRPC connection options if necessary.
	// NOTE: use with care, this overrides the default dialing options including the
	// interceptors for authentication!
	Dialing []grpc.DialOption

	// The URL of the Quarterdeck system for authentication; by default AuthEndpoint.
	AuthURL string

	// If true, the client will not use TLS to connect to Ensign (default false).
	Insecure bool

	// If true, the client will not login with the api credentials and will omit access
	// tokens from Ensign RPCs. This is primarily used for testing against mocks.
	NoAuthentication bool

	// Mocking allows the client to be used in test code. Set testing mode to true and
	// create a *mock.Ensign to add to the dialer. Any other dialer options can also be
	// added to the mock for connection purposes.
	Testing bool
	Mock    *mock.Ensign
}

// NewOptions instantiates an options object for configuring Ensign, sets defaults and
// loads missing options from the environment, then validates the options; returning an
// error if the options are incorrectly configured.
func NewOptions(opts ...Option) (options Options, err error) {
	options = Options{}
	for _, opt := range opts {
		if err = opt(&options); err != nil {
			return Options{}, err
		}
	}

	if err = options.Validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

// Validate the options to make sure required configuration is set. This method also
// ensures that default values are set if a configuration is missing. For example, if
// the Endpoint is not set, this method first tries to set it from the environment, and
// then uses the default value as a last step.
func (o *Options) Validate() (err error) {
	o.setDefaults()

	// If in testing mode, all we need is a mock object and nothing else.
	if o.Testing {
		if o.Mock == nil {
			return ErrMissingMock
		}
		return nil
	}

	if o.Endpoint == "" {
		return ErrMissingEndpoint
	}

	if !o.NoAuthentication {
		if o.ClientID == "" {
			return ErrMissingClientID
		}

		if o.ClientSecret == "" {
			return ErrMissingClientSecret
		}

		if o.AuthURL == "" {
			return ErrMissingAuthURL
		}
	}
	return nil
}

// Set defaults from the environment and then from any applicable constants.
func (o *Options) setDefaults() {
	// Set the client ID from the environment
	if o.ClientID == "" {
		o.ClientID = os.Getenv(EnvClientID)
	}

	// Set the client Secret from the environment
	if o.ClientSecret == "" {
		o.ClientSecret = os.Getenv(EnvClientSecret)
	}

	// Set the endpoint from the environment or from the default.
	if o.Endpoint == "" {
		if o.Endpoint = os.Getenv(EnvEndpoint); o.Endpoint == "" {
			o.Endpoint = EnsignEndpoint
		}
	}

	// Set the auth url from the environment or from the default.
	if o.AuthURL == "" {
		if o.AuthURL = os.Getenv(EnvAuthURL); o.AuthURL == "" {
			o.AuthURL = AuthEndpoint
		}
	}

	// Set insecure from the environment if it's not already set to true.
	if !o.Insecure {
		if envs, ok := os.LookupEnv(EnvInsecure); ok {
			o.Insecure = parseBool(envs, false)
		}
	}

	// Set no authentication from the environment if it's not already set to true.
	if !o.NoAuthentication {
		if envs, ok := os.LookupEnv(EnvNoAuth); ok {
			o.NoAuthentication = parseBool(envs, false)
		}
	}
}

func parseBool(s string, defaultValue bool) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "1", "y", "t", "yes", "true", "on":
		return true
	case "", "0", "f", "n", "no", "false", "off":
		return false
	default:
		return defaultValue
	}
}
