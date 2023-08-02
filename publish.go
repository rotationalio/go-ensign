package ensign

import (
	"context"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/stream"
	"google.golang.org/grpc"
)

// Publish one or more events to the specified topic name or topic ID. The first time
// that Publish is called, a Publisher stream is opened by the client that will run in
// its own go routine for the duration to the program; if the publish stream cannot be
// opened an error is returned. Otherwise, each event passed to the publish method will
// be sent to Ensign. If the Ensign connection has dropped or another connection error
// occurs an error will be returned. Once the event is published, it is up to the user
// to listen for an Ack or Nack on each event to determine if the event was specifically
// published or not.
func (c *Client) Publish(topic string, events ...*Event) (err error) {
	// Ensure the publisher is open before publishing
	c.openPub.Do(func() {
		c.pub, err = stream.NewPublisher(c, c.copts...)
	})

	// If the publisher could not be opened, return an error
	if err != nil {
		return err
	}

	// Attempt to send all events to the server, stopping on the first error.
	for _, event := range events {
		// Publish the event and collect the event info and reply channel.
		if event.info, event.pub, err = c.pub.Publish(topic, event.Proto()); err != nil {
			return err
		}

		// Ensure the event state is set to published.
		event.state = published
	}
	return nil
}

// PublishStream allows you to open a gRPC stream server to ensign for publishing API
// events directly. This manual mechanism of opening a stream is for advanced users and
// is not recommended in production. Instead using Publish or CreatePublisher is the
// best way to establish a stream connection to Ensign.
func (c *Client) PublishStream(ctx context.Context, opts ...grpc.CallOption) (api.Ensign_PublishClient, error) {
	return c.api.Publish(ctx, opts...)
}
