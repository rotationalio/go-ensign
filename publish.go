package ensign

import (
	"context"

	"github.com/rotationalio/go-ensign/stream"
)

func (c *Client) Publish(topic string, events ...*Event) (err error) {
	if c.pub == nil {
		if c.pub, err = stream.Publish(c.api, context.TODO(), c.copts...); err != nil {
			return err
		}
	}

	for _, event := range events {
		c.pub.Publish(topic, event.toPB())
	}
	return nil
}
