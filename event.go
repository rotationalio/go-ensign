package ensign

import (
	"time"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Events wrap user-defined datagrams that are totally ordered by the Ensign platform.
// Publishers create events with arbitrary data and send them to Ensign so that they can
// be sent to Subscribers awaiting the events or queried using EnSQL for later
// consumption. The datagram the event wraps is user-specific. It can be JSON, msgpack,
// text data, parquet, protocol buffers, etc. Applications should define event types
// using the Ensign schema registry and use those types to create events to publish and
// subscribe/query from.
type Event struct {
	// Users can specify an ID that can be used locally for tracking the event but
	// cannot be used globally accross multiple publishers and subscribers. This ID is
	// empty by default and not used by Ensign except for logging and debugging.
	LocalID string

	// Metadata are user-defined key/value pairs that can be optionally added to an
	// event to store/lookup data without unmarshaling the entire payload.
	Metadata Metadata

	// Data is the datagram payload that defines the event.
	Data []byte

	// Mimetype describes how to parse the event datagram.
	Mimetype mimetype.MIME

	// Type defines the schema of the event datagram and is optional.
	Type *api.Type

	// Created is the timestamp that the event was created according to the client clock.
	Created time.Time
}

// Convert an event into a protocol buffer event.
func (e *Event) toPB() *api.Event {
	return &api.Event{
		UserDefinedId: e.LocalID,
		Data:          e.Data,
		Metadata:      map[string]string(e.Metadata),
		Mimetype:      e.Mimetype,
		Type:          e.Type,
		Created:       timestamppb.New(e.Created),
	}
}

// Convert a protocol buffer event into this event.
func (e *Event) fromPB(event *api.Event) {
	e.LocalID = event.UserDefinedId
	e.Data = event.Data
	e.Metadata = Metadata(event.Metadata)
	e.Mimetype = event.Mimetype
	e.Type = event.Type
	e.Created = event.Created.AsTime()
}

// Metadata are user-defined key/value pairs that can be optionally added to an
// event to store/lookup data without unmarshaling the entire payload.
type Metadata map[string]string

// Get returns the metadata value for the given key. If the key is not in the metadata
// an empty string is returned without an error.
func (m Metadata) Get(key string) string {
	if val, ok := m[key]; ok {
		return val
	}
	return ""
}

// Set a metadata value for the given key; overwrites existing keys.
func (m Metadata) Set(key, value string) {
	m[key] = value
}
