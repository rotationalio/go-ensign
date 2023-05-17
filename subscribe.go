package ensign

import (
	"context"
	"errors"

	"github.com/rotationalio/go-ensign/stream"
)

type Subscription struct {
	C      <-chan *Event
	stream stream.Subscriber
}

func (c *Client) Subscribe(ctx context.Context, topics ...string) (sub *Subscription, err error) {
	return nil, errors.New("not implemented")
	// 	events := make(chan *Event, 1)
	// 	sub = &Subscription{C: events}

	// 	if sub.stream, err = stream.Subscribe(c.api, ctx, topics, c.copts...); err != nil {
	// 		return nil, err
	// 	}

	// 	var in <-chan *api.EventWrapper
	// 	if in, err = sub.stream.Subscribe(); err != nil {
	// 		return nil, err
	// 	}

	// 	// TODO: map topic names to IDs
	// 	// TODO: handle consumer groups
	// 	// TODO: handle events coming from the subscription stream
	// 	go func(out chan<- *Event, in <-chan *api.EventWrapper) {
	// 		for wrapper := range in {
	// 			// Convert the event into an API event
	// 			// TODO: handle the subscribe request channel
	// 			event := &Event{}
	// 			if _, err := event.fromPB(wrapper, subscription); err != nil {
	// 				// TODO: what to do about the error?
	// 				panic(err)
	// 			}

	// 			out <- event
	// 		}
	// 	}(events, in)

	// 	return sub, nil
}

func (c *Subscription) Close() error {
	return c.stream.Close()
}
