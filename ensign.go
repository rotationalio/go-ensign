package sdk

import (
	"context"
	"crypto/tls"
	"io"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const BufferSize = 128

// Client manages the credentials and connection to the ensign server.
type Client struct {
	opts *Options
	cc   *grpc.ClientConn
	api  api.EnsignClient
	auth *auth.Client
}

// Publisher is a low level interface for sending events to a topic or a group of topics
// that have been defined in Ensign services.
type Publisher interface {
	io.Closer
	Errorer
	Publish(events ...*api.Event)
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
		var creds credentials.PerRPCCredentials
		if creds, err = c.auth.Login(context.Background(), c.opts.ClientID, c.opts.ClientSecret); err != nil {
			return err
		}

		opts = append(opts, grpc.WithPerRPCCredentials(creds))
	}

	if c.cc, err = grpc.Dial(c.opts.Endpoint, opts...); err != nil {
		return err
	}

	c.api = api.NewEnsignClient(c.cc)
	return nil
}

func (c *Client) Close() (err error) {
	defer func() {
		c.cc = nil
		c.api = nil
	}()

	if err = c.cc.Close(); err != nil {
		return err
	}
	return nil
}

func (c *Client) Publish(ctx context.Context) (_ Publisher, err error) {
	pub := &publisher{
		send: make(chan *api.Event, BufferSize),
		recv: make(chan *api.Publication, BufferSize),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}
	if pub.stream, err = c.api.Publish(ctx); err != nil {
		return nil, err
	}

	// Start go routines
	pub.wg.Add(2)
	go pub.sender()
	go pub.recver()

	return pub, nil
}

func (c *Client) Subscribe(ctx context.Context) (_ Subscriber, err error) {
	sub := &subscriber{
		send: make(chan *api.Subscription, BufferSize),
		recv: make([]chan<- *api.Event, 0, 1),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}

	if sub.stream, err = c.api.Subscribe(ctx); err != nil {
		return nil, err
	}

	// Start go routines
	sub.wg.Add(2)
	go sub.sender()
	go sub.recver()

	return sub, nil
}
