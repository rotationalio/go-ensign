package stream

import (
	"context"
	"sync"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc"
)

type subscriber struct {
	sync.RWMutex
	stream api.Ensign_SubscribeClient
	send   chan *api.SubscribeRequest
	recv   []chan<- *api.EventWrapper
	stop   chan struct{}
	wg     sync.WaitGroup
	errc   chan error
}

var _ Subscriber = &subscriber{}

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

func (c *subscriber) Subscribe() (<-chan *api.EventWrapper, error) {
	sub := make(chan *api.EventWrapper, BufferSize)
	c.Lock()
	defer c.Unlock()
	c.recv = append(c.recv, sub)
	return sub, nil
}

func (c *subscriber) Ack(id []byte) error {
	c.send <- &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Ack{
			Ack: &api.Ack{
				Id: id,
			},
		},
	}
	return nil
}

func (c *subscriber) Nack(id []byte, err error) error {
	nack := &api.Nack{
		Id: id,
	}
	if err != nil {
		nack.Error = err.Error()
	}

	c.send <- &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Nack{
			Nack: nack,
		},
	}
	return nil
}

func (c *subscriber) Err() error {
	select {
	case err := <-c.errc:
		return err
	default:
	}
	return nil
}

func (c *subscriber) Close() error {
	// Cannot call CloseSend concurrently with send message.
	// Send stop signals to sender and recver routines
	c.stop <- struct{}{}
	close(c.send)

	c.wg.Wait()
	for _, sub := range c.recv {
		close(sub)
	}
	return c.stream.CloseSend()
}

func (c *subscriber) sender() {
	defer c.wg.Done()
	for e := range c.send {
		if err := c.stream.Send(e); err != nil {
			c.errc <- err
			return
		}
	}
}

func (c *subscriber) recver() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stop:
			return
		default:
		}

		e, err := c.stream.Recv()
		if err != nil {
			c.errc <- err
			return
		}

		// Fetch the event from the subscribe reply
		// TODO: handle other message types such as close stream
		var wrapper *api.EventWrapper
		if wrapper = e.GetEvent(); wrapper == nil {
			continue
		}

		c.RLock()
		for _, sub := range c.recv {
			sub <- wrapper
		}
		c.RUnlock()
	}
}
