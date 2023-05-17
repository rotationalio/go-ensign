package ensign

import (
	"context"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/stream"
	"google.golang.org/grpc"
)

type Subscription struct {
	C      <-chan *Event
	events <-chan *api.EventWrapper
	stream *stream.Subscriber
}

func (c *Client) Subscribe(topics ...string) (sub *Subscription, err error) {
	// Create the internal subscription stream
	sub = &Subscription{}
	if sub.events, sub.stream, err = stream.NewSubscriber(c, topics, c.copts...); err != nil {
		return nil, err
	}

	// Create the user events channel
	out := make(chan *Event, 1)
	sub.C = out

	// Run the subscription background go routine
	go sub.eventHandler(out)
	return sub, nil
}

func (c *Subscription) Close() error {
	return c.stream.Close()
}

func (c *Subscription) eventHandler(out chan<- *Event) {
	for wrapper := range c.events {
		// Convert the event into an API event
		event := &Event{}
		if err := event.fromPB(wrapper, subscription); err != nil {
			// TODO: what to do about the error?
			panic(err)
		}

		// Attach the stream to send acks/nacks back
		event.sub = c.stream
		out <- event
	}
}

func (c *Client) SubscribeStream(ctx context.Context, opts ...grpc.CallOption) (api.Ensign_SubscribeClient, error) {
	return c.api.Subscribe(ctx, opts...)
}
