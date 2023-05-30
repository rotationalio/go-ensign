package stream_test

import (
	"context"
	"time"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// MockConnectionObserver implements ConnectionObserver, PublishClient, and
// SubscribeClient to connect tests to a mock Ensign server via a bufconn.
type MockConnectionObserver struct {
	server *mock.Ensign
	sock   *mock.Listener
	conn   *grpc.ClientConn
	client api.EnsignClient
}

func NewMockConnectionObserver() (_ *MockConnectionObserver, err error) {
	conn := mock.NewBufConn()

	var cc *grpc.ClientConn
	if cc, err = conn.Connect(context.Background(), grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		return nil, err
	}

	return &MockConnectionObserver{
		server: mock.New(conn),
		conn:   cc,
		sock:   conn,
		client: api.NewEnsignClient(cc),
	}, nil
}

func (c *MockConnectionObserver) ConnState() connectivity.State {
	return c.conn.GetState()
}

// WaitForReconnect checks if the connection has been reconnected periodically and
// retruns true when the connection is ready. If the context deadline timesout before
// a connection can be re-established, false is returned.
//
// Experimental: this method relies on an experimental gRPC API that could be changed.
func (c *MockConnectionObserver) WaitForReconnect(ctx context.Context) bool {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Connect causes all subchannels in the ClientConn to attempt to connect if
			// the channel is idle. Does not wait for the connection attempts to begin.
			c.conn.Connect()

			// Check if the connection is ready
			if c.conn.GetState() == connectivity.Ready {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

func (c *MockConnectionObserver) PublishStream(ctx context.Context, opts ...grpc.CallOption) (api.Ensign_PublishClient, error) {
	return c.client.Publish(ctx, opts...)
}

func (c *MockConnectionObserver) SubscribeStream(ctx context.Context, opts ...grpc.CallOption) (api.Ensign_SubscribeClient, error) {
	return c.client.Subscribe(ctx, opts...)
}

func CheckStatusError(require *require.Assertions, err error, code codes.Code, message string, msgAndArgs ...interface{}) {
	require.Error(err, msgAndArgs...)

	serr, ok := status.FromError(err)
	require.True(ok, msgAndArgs...)

	require.Equal(code, serr.Code(), msgAndArgs...)
	if message != "" {
		require.Equal(message, serr.Message(), msgAndArgs...)
	}
}
