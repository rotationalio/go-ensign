package stream

import (
	"io"
	"sync"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

type publisher struct {
	stream api.Ensign_PublishClient
	send   chan *api.EventWrapper
	recv   chan *api.PublisherReply
	stop   chan struct{}
	wg     sync.WaitGroup
	errc   chan error
}

var _ Publisher = &publisher{}

func (c *publisher) Publish(topic string, events ...*api.Event) {
	// TODO: handle topic name mapping and topic manager
	// TODO: better error handling
	topicID, _ := ulid.Parse(topic)

	for _, event := range events {
		// TODO: handle topic name mapping
		env := &api.EventWrapper{
			TopicId: topicID.Bytes(),
		}

		// TODO: handle errors
		env.Wrap(event)

		c.send <- env
	}
}

func (c *publisher) Err() error {
	select {
	case err := <-c.errc:
		return err
	default:
		return nil
	}
}

func (c *publisher) Close() error {
	// Cannot call CloseSend concurrently with send message.
	// Send stop signals to sender and recver go routines
	close(c.send)
	c.stop <- struct{}{}

	c.wg.Wait()
	close(c.recv)
	return c.stream.CloseSend()
}

func (c *publisher) sender() {
	defer c.wg.Done()
	for e := range c.send {
		if err := c.stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_Event{Event: e}}); err != nil {
			c.errc <- err
			return
		}
	}
}

func (c *publisher) recver() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stop:
			return
		default:
		}

		_, err := c.stream.Recv()
		if err != nil && err != io.EOF {
			c.errc <- err
			return
		}

		// Just drop acks for now
		// TODO: handle publish acks from the ensign server
		// c.recv <- ack
	}
}
