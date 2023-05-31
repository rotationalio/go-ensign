package ensign_test

import (
	"math/rand"
	"time"

	"github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
)

// NewEvent returns a new random event for testing purposes.
func NewEvent() *ensign.Event {
	event := &ensign.Event{
		Metadata: ensign.Metadata{"length": "256"},
		Data:     make([]byte, 256),
		Mimetype: mimetype.ApplicationOctetStream,
		Type: &api.Type{
			Name:         "random",
			MajorVersion: 1,
		},
		Created: time.Now(),
	}

	rand.Read(event.Data)
	return event
}
