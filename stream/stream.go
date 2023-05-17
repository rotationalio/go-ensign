/*
Package stream provides
*/
package stream

import (
	"context"
	"io"
	"time"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

const (
	BufferSize       = 128
	ReconnectTimeout = 5 * time.Minute
)

type ConnectionObserver interface {
	ConnState() connectivity.State
	WaitForReconnect(ctx context.Context) bool
}

type PublishClient interface {
	ConnectionObserver
	PublishStream(context.Context, ...grpc.CallOption) (api.Ensign_PublishClient, error)
}

type Subscriber interface {
	io.Closer
	Errorer
	Subscribe() (<-chan *api.EventWrapper, error)
	Ack(id []byte) error
	Nack(id []byte, err error) error
}

type SubscribeClient interface {
	ConnectionObserver
	Subscribe(context.Context, ...grpc.CallOption) (api.Ensign_SubscribeClient, error)
}
