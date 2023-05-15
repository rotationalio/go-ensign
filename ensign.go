package ensign

import (
	"context"
	"crypto/tls"
	"sync"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Client manages the credentials and connection to the Ensign server.
type Client struct {
	sync.RWMutex
	opts  Options
	cc    *grpc.ClientConn
	api   api.EnsignClient
	auth  *auth.Client
	copts []grpc.CallOption
	pub   stream.Publisher
}

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
