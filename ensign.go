package ensign

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// Specifies the wait period before checking if a gRPC connection has been
	// established while waiting for a ready connection.
	ReconnectTick = 750 * time.Millisecond

	// The default page size for paginated gRPC responses.
	DefaultPageSize = uint32(100)
)

// Client manages the credentials and connection to the Ensign server. The New() method
// creates a configured client and the Client methods are used to interact with the
// Ensign ecosystem, handling authentication, publish and subscribe streams, and
// interactions with topics. The Ensign client is the top-level method for creating Go
// applications that leverage data flows.
type Client struct {
	sync.RWMutex
	opts    Options
	cc      *grpc.ClientConn
	api     api.EnsignClient
	auth    *auth.Client
	copts   []grpc.CallOption
	pub     *stream.Publisher
	openPub sync.Once
}

// Create a new Ensign client, specifying connection and authentication options if
// necessary. Ensign expects that credentials are stored in the environment, set using
// the $ENSIGN_CLIENT_ID and $ENSIGN_CLIENT_SECRET environment variables. They can also
// be set manually using the WithCredentials or WithLoadCredentials options. You can
// also specify a mock ensign server to test your code that uses Ensign via WithMock.
// This function returns an error if the client is unable to dial ensign; however,
// authentication errors and connectivity checks may require an Ensign RPC call. You can
// use the Ping() method to check if your connection credentials to Ensign is correct.
func New(opts ...Option) (client *Client, err error) {
	client = &Client{}
	if client.opts, err = NewOptions(opts...); err != nil {
		return nil, err
	}

	// Connect to the authentication service -- this must happen before the connection
	// to the ensign server so that the client-side interceptors can be created.
	if !client.opts.NoAuthentication {
		if client.auth, err = auth.New(client.opts.AuthURL, client.opts.Insecure); err != nil {
			return nil, err
		}
	}

	// If in testing mode, connect to the mock and stop connecting.
	if client.opts.Testing {
		if err = client.connectMock(); err != nil {
			return nil, err
		}
		return client, nil
	}

	// If not in testing mode, connect to the Ensign server.
	if err = client.connect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) connect() (err error) {
	// Fetch the dialing options from the ensign config.
	opts := make([]grpc.DialOption, 0, 3)
	opts = append(opts, c.opts.Dialing...)

	// If no dialing opts were specified create default dialing options.
	if len(opts) == 0 {
		if c.opts.Insecure {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}

		if !c.opts.NoAuthentication {
			// Rather than using the PerRPC Dial Option add interceptors that ensure the
			// access and refresh token are valid on every RPC call, and that
			// reauthenticate with Quarterdeck when access tokens expire.
			// NOTE: must ensure that we login first!
			if _, err = c.auth.Login(context.Background(), c.opts.ClientID, c.opts.ClientSecret); err != nil {
				return err
			}

			opts = append(opts, grpc.WithUnaryInterceptor(c.auth.UnaryAuthenticate))
			opts = append(opts, grpc.WithStreamInterceptor(c.auth.StreamAuthenticate))
		}
	}

	if c.cc, err = grpc.Dial(c.opts.Endpoint, opts...); err != nil {
		return err
	}

	c.api = api.NewEnsignClient(c.cc)
	return nil
}

func (c *Client) connectMock() (err error) {
	if !c.opts.Testing || c.opts.Mock == nil {
		return ErrMissingMock
	}

	if c.api, err = c.opts.Mock.Client(context.Background(), c.opts.Dialing...); err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() (err error) {
	c.Lock()
	defer func() {
		c.cc = nil
		c.api = nil
		c.Unlock()
	}()

	if c.cc != nil {
		if err = c.cc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Status(ctx context.Context) (state *api.ServiceState, err error) {
	return c.api.Status(ctx, &api.HealthCheck{}, c.copts...)
}

// WithCallOptions configures the next client Call to use the specified call options,
// after the call, the call options are removed. This method returns the Client pointer
// so that you can easily chain a call e.g. client.WithCallOptions(opts...).ListTopics()
// -- this ensures that we don't have to pass call options in to each individual call.
// Ensure that the clone of the client is discarded and garbage collected after use;
// the clone cannot be used to close the connection or fetch the options.
//
// Experimental: call options and thread-safe cloning is an experimental feature and its
// signature may be subject to change in the future.
func (c *Client) WithCallOptions(opts ...grpc.CallOption) *Client {
	// Return a clone of the client with the api interface and the opts but do not
	// include the grpc connection to ensure only the original client can close it.
	client := &Client{
		opts:  c.opts,
		api:   c.api,
		auth:  c.auth,
		copts: opts,
	}
	return client
}

func (c *Client) EnsignClient() api.EnsignClient {
	return c.api
}

func (c *Client) QuarterdeckClient() *auth.Client {
	return c.auth
}

// Conn state returns the connectivity state of the underlying gRPC connection.
//
// Experimental: this method relies on an experimental gRPC API that could be changed.
func (c *Client) ConnState() connectivity.State {
	return c.cc.GetState()
}

// Wait for the state of the underlying gRPC connection to change from the source state
// (not to the source state) or until the context times out. Returns true if the source
// state has changed to another state.
//
// Experimental: this method relies on an experimental gRPC API that could be changed.
func (c *Client) WaitForConnStateChange(ctx context.Context, sourceState connectivity.State) bool {
	return c.cc.WaitForStateChange(ctx, sourceState)
}

// WaitForReconnect checks if the connection has been reconnected periodically and
// retruns true when the connection is ready. If the context deadline timesout before
// a connection can be re-established, false is returned.
//
// Experimental: this method relies on an experimental gRPC API that could be changed.
func (c *Client) WaitForReconnect(ctx context.Context) bool {
	ticker := time.NewTicker(ReconnectTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Connect causes all subchannels in the ClientConn to attempt to connect if
			// the channel is idle. Does not wait for the connection attempts to begin.
			c.cc.Connect()

			// Check if the connection is ready
			if c.cc.GetState() == connectivity.Ready {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}
