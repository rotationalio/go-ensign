package ensign

import (
	"context"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/stream"
	"google.golang.org/grpc"
)

// A Subscription object with a channel of events is returned when you subscribe to a
// topic or topics. Listen on the provided channel in order to receive events from
// Ensign when they are published to your consumer group. It is the user's
// responsibility to Ack and Nack events when they are handled by using the methods on
// the event itself.
type Subscription struct {
	C      <-chan *Event
	events <-chan *api.EventWrapper
	stream *stream.Subscriber
}

// Subscribe creates a subscription stream to the specified topics and returns a
// Subscription with a channel that can be listened on for incoming events. If the
// client cannot connect to Ensign or a subscription stream cannot be established, an
// error is returned.
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

// Close the subscription stream and associated channels, preventing any more events
// from being received and signaling to handler code that no more events will arrive.
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

// SubscribeStream allows you to open a gRPC stream server to ensign for subscribing to
// API events directly. This manual mechanism of opening a stream is for advanced users
// and is not recommended in production. Instead using Subscribe or CreateSubscriber is
// the best way to establish a stream connection to Ensign.
func (c *Client) SubscribeStream(ctx context.Context, opts ...grpc.CallOption) (api.Ensign_SubscribeClient, error) {
	return c.api.Subscribe(ctx, opts...)
}
