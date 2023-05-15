package stream

import (
	"context"
	"io"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc"
)

const BufferSize = 128

// Publisher is a low level interface for sending events to a topic or a group of topics
// that have been defined in Ensign services.
type Publisher interface {
	io.Closer
	Errorer
	Publish(topic string, events ...*api.Event)
}

type PublishClient interface {
	Publish(context.Context, ...grpc.CallOption) (api.Ensign_PublishClient, error)
}

type Subscriber interface {
	io.Closer
	Errorer
	Subscribe() (<-chan *api.EventWrapper, error)
	Ack(id []byte) error
	Nack(id []byte, err error) error
}

type SubscribeClient interface {
	Subscribe(context.Context, ...grpc.CallOption) (api.Ensign_SubscribeClient, error)
}

func Publish(client PublishClient, ctx context.Context, opts ...grpc.CallOption) (_ Publisher, err error) {
	pub := &publisher{
		send: make(chan *api.EventWrapper, BufferSize),
		recv: make(chan *api.PublisherReply, BufferSize),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}

	// Connect to the stream and send the open stream request
	if pub.stream, err = client.Publish(ctx, opts...); err != nil {
		return nil, err
	}

	// TODO: should we send topics from the topic cache?
	open := &api.OpenStream{
		ClientId: ulid.Make().String(),
	}

	if err = pub.stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_OpenStream{OpenStream: open}}); err != nil {
		return nil, err
	}

	// TODO: handle the topic map returned from the server
	var rep *api.PublisherReply
	if rep, err = pub.stream.Recv(); err != nil {
		return nil, err
	}

	if ready := rep.GetReady(); ready == nil {
		return nil, ErrStreamUninitialized
	}

	// Start go routines
	pub.wg.Add(2)
	go pub.sender()
	go pub.recver()

	return pub, nil
}

func Subscribe(client SubscribeClient, ctx context.Context, topics []string, opts ...grpc.CallOption) (_ Subscriber, err error) {
	sub := &subscriber{
		send: make(chan *api.SubscribeRequest, BufferSize),
		recv: make([]chan<- *api.EventWrapper, 0, 1),
		stop: make(chan struct{}, 1),
		errc: make(chan error, 1),
	}

	// Connect to the stream and send stream policy information
	if sub.stream, err = client.Subscribe(ctx, opts...); err != nil {
		return nil, err
	}

	// TODO: map topic names to IDs
	// TODO: handle consumer groups here
	open := &api.Subscription{
		ClientId: ulid.Make().String(),
		Topics:   topics,
	}

	if err = sub.stream.Send(&api.SubscribeRequest{Embed: &api.SubscribeRequest_Subscription{Subscription: open}}); err != nil {
		return nil, err
	}

	// TODO: handle the topic map returned from the server
	var rep *api.SubscribeReply
	if rep, err = sub.stream.Recv(); err != nil {
		return nil, err
	}

	if ready := rep.GetReady(); ready == nil {
		return nil, ErrStreamUninitialized
	}

	// Start go routines
	sub.wg.Add(2)
	go sub.sender()
	go sub.recver()

	return sub, nil
}
