package sdk

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const BufferSize = 128

// Client manages the credentials and connection to the ensign server.
type Client struct {
	opts  *Options
	cc    *grpc.ClientConn
	api   api.EnsignClient
	auth  *auth.Client
	copts []grpc.CallOption
}

// Publisher is a low level interface for sending events to a topic or a group of topics
// that have been defined in Ensign services.
type Publisher interface {
	io.Closer
	Errorer
	Publish(topic string, events ...*api.Event)
}

type Subscriber interface {
	io.Closer
	Errorer
	Subscribe() (<-chan *api.Event, error)
	Ack(id string) error
	Nack(id string, err error) error
}

func New(opts *Options) (client *Client, err error) {
	if opts == nil {
		opts = NewOptions()
	}

	if err = opts.Validate(); err != nil {
		return nil, err
	}

	client = &Client{opts: opts}

	if client.auth, err = auth.New(opts.AuthURL, opts.Insecure); err != nil {
		return nil, err
	}

	if err = client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) Connect(opts ...grpc.DialOption) (err error) {
	if len(opts) == 0 {
		opts = make([]grpc.DialOption, 0, 2)
		if c.opts.Insecure {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}
	}

	if !c.opts.NoAuthentication {
		// Rather than using the PerRPC Dial Option add interceptors that ensure the
		// access and refresh token are valid on every RPC call, reauthenticating with
		// Quarterdeck as necessary. NOTE: must ensure that we login first!
		if _, err = c.auth.Login(context.Background(), c.opts.ClientID, c.opts.ClientSecret); err != nil {
			return err
		}

		opts = append(opts, grpc.WithUnaryInterceptor(c.auth.UnaryAuthenticate))
		opts = append(opts, grpc.WithStreamInterceptor(c.auth.StreamAuthenticate))
	}

	if c.cc, err = grpc.Dial(c.opts.Endpoint, opts...); err != nil {
		return err
	}

	c.api = api.NewEnsignClient(c.cc)
	return nil
}

func (c *Client) ConnectMock(mock *mock.Ensign, opts ...grpc.DialOption) (err error) {
	if c.api, err = mock.Client(context.Background(), opts...); err != nil {
		return err
	}
	return nil
}

func (c *Client) ConnectAuth(auth *auth.Client) error {
	c.auth = auth
	return nil
}

func (c *Client) Close() (err error) {
	defer func() {
		c.cc = nil
		c.api = nil
	}()

	if c.cc != nil {
		if err = c.cc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Publish(ctx context.Context) (_ Publisher, err error) {
	defer c.resetCallOpts()

	pub := &publisher{
		send: make(chan *api.Event, BufferSize),
		recv: make(chan *api.Publication, BufferSize),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}
	if pub.stream, err = c.api.Publish(ctx, c.copts...); err != nil {
		return nil, err
	}

	// Start go routines
	pub.wg.Add(2)
	go pub.sender()
	go pub.recver()

	return pub, nil
}

func (c *Client) Subscribe(ctx context.Context, topics ...string) (_ Subscriber, err error) {
	defer c.resetCallOpts()

	sub := &subscriber{
		send: make(chan *api.Subscription, BufferSize),
		recv: make([]chan<- *api.Event, 0, 1),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}

	// Connect to the stream and send stream policy information
	if sub.stream, err = c.api.Subscribe(ctx, c.copts...); err != nil {
		return nil, err
	}

	// TODO: map topic names to IDs
	// TODO: handle consumer groups here
	open := &api.OpenStream{
		ConsumerId: ulid.Make().String(),
		Topics:     topics,
	}

	if err = sub.stream.Send(&api.Subscription{Embed: &api.Subscription_OpenStream{OpenStream: open}}); err != nil {
		return nil, err
	}

	// TODO: perform a recv to ensure the stream has been successfully connected to

	// Start go routines
	sub.wg.Add(2)
	go sub.sender()
	go sub.recver()

	return sub, nil
}

// WithCallOptions configures the next client Call to use the specified call options,
// after the call, the call options are removed. This method returns the Client pointer
// so that you can easily chain a call e.g. client.WithCallOptions(opts...).ListTopics()
// -- this ensures that we don't have to pass call options in to each individual call.
func (c *Client) WithCallOptions(opts ...grpc.CallOption) *Client {
	c.copts = opts
	return c
}

func (c *Client) resetCallOpts() {
	c.copts = nil
}

func (c *Client) EnsignClient() api.EnsignClient {
	return c.api
}

func (c *Client) QuarterdeckClient() *auth.Client {
	return c.auth
}
