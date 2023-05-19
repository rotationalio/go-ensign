/*
Package stream provides
*/
package stream

import (
	"context"
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

type SubscribeClient interface {
	ConnectionObserver
	SubscribeStream(context.Context, ...grpc.CallOption) (api.Ensign_SubscribeClient, error)
}
