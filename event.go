package ensign

import (
	"bytes"
	"context"
	"fmt"
	"sync"
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

	// Internal fields used for managing the event through the publish or subscribe
	// workflows. The goal of the public facing parts of the event is to give the user
	// an easy tool to work with events while abstracting Ensign eventing details.
	mu    sync.Mutex
	state eventState
	info  *api.EventWrapper
	ctx   context.Context
	err   error
	pub   <-chan *api.PublisherReply
	sub   chan<- *api.SubscribeRequest
}

type eventState uint8

const (
	initialized  eventState = iota // event has been created but hasn't been published
	published                      // event has been published, awaiting ack from server
	subscription                   // event has been received from subscription, awaiting ack from user
	acked                          // event has been acked from user or server
	nacked                         // event has been nacked from user or server
)

// Returns the event ID if the event has been published; otherwise returns nil.
func (e *Event) ID() []byte {
	if e.info != nil && len(e.info.Id) > 0 {
		return e.info.Id
	}
	return nil
}

// Returns the topic ID that the event was published to if available; otherwise returns nil.
func (e *Event) TopicID() []byte {
	if e.info != nil && len(e.info.TopicId) > 0 {
		return e.info.TopicId
	}
	return nil
}

// Returns the offset and epoch of the event if available, otherwise returns 0.
func (e *Event) Offset() (offset uint64, epoch uint64) {
	if e.info != nil {
		return e.info.Offset, e.info.Epoch
	}
	return 0, 0
}

// Returns the committed timestamp if available.
func (e *Event) Committed() time.Time {
	if e.info != nil && e.info.Committed != nil {
		return e.info.Committed.AsTime()
	}
	return time.Time{}
}

// Acked allows a user to check if an event published to an event stream has been
// successfully received by the server.
func (e *Event) Acked() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check the publisher reply stream to see if an ack or nack has been received.
	if e.state == published {
		e.checkpub()
	}

	return e.state == acked, e.err
}

// Nacked allows a user to check if an event published to an event stream has errored or
// otherwise been rejected by the server.
func (e *Event) Nacked() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check the publisher reply stream to see if an ack or nack has been received.
	if e.state == published {
		e.checkpub()
	}

	return e.state == nacked, e.err
}

func (e *Event) checkpub() {
	select {
	case rep := <-e.pub:
		switch msg := rep.Embed.(type) {
		case *api.PublisherReply_Ack:
			e.state = acked
			e.info.Id = msg.Ack.Id
			e.info.Committed = msg.Ack.Committed
		case *api.PublisherReply_Nack:
			e.state = nacked
			e.err = makeNackError(msg.Nack)
		default:
			e.err = fmt.Errorf("unhandled publisher reply %T", rep.Embed)
		}
	default:
	}
}

// Ack allows a user to acknowledge back to the Ensign server that an event received by
// a subscription stream has been successfully consumed. For consumer groups that have
// exactly-once or at-least-once semantics, this signals the message has been delivered
// successfully so as to not trigger a redelivery of the message to another consumer.
// Ack does not block and returns true if already acked. If a nack was sent before ack,
// then this method returns false. If this event was not received on a subscribe stream
// then an error is returned.
func (e *Event) Ack() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch e.state {
	case acked:
		return true, e.err
	case nacked:
		return false, e.err
	case initialized, published:
		return false, ErrCannotAck
	}

	// Send the ack on the sub channel to be sent back to the Ensign server?
	e.sub <- &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Ack{
			Ack: &api.Ack{
				Id: e.info.Id,
			},
		},
	}
	close(e.sub)

	// TODO: what happens if the ack message cannot be sent?
	e.state = acked
	return true, nil
}

// Nack allows a user to signal to the Ensign server that an event received by a
// subscription stream has not been successfully consumed. For consumer groups that have
// exactly-once or at-least-once semantics, this signals the message needs to be
// redelivered to another consumer.
//
// Nack does not block and returns true if already nacked. If an ack was sent before
// the nack, then this method returns false. If this event was not received on a
// subscribe stream then an error is returned.
func (e *Event) Nack(code api.Nack_Code) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch e.state {
	case nacked:
		return true, e.err
	case acked:
		return false, e.err
	case initialized, published:
		return false, ErrCannotAck
	}

	// Send the ack on the sub channel to be sent back to the Ensign server?
	e.sub <- &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Nack{
			Nack: &api.Nack{
				Id:   e.info.Id,
				Code: code,
			},
		},
	}
	close(e.sub)

	// TODO: what happens if the ack message cannot be sent?
	e.state = acked
	return true, nil
}

// Err returns any error that occurred processing the event.
func (e *Event) Err() error {
	return e.err
}

// Context returns the message context if set otherwise a background context.
func (e *Event) Context() context.Context {
	if e.ctx != nil {
		return e.ctx
	}
	return context.Background()
}

// SetContext provides an event context for use in the handling application.
func (e *Event) SetContext(ctx context.Context) {
	e.ctx = ctx
}

// Clone the event, resetting its state and removing acks, nacks, created timestamp and
// context. Useful for resending events or for duplicating an event to edit and publish.
func (e *Event) Clone() *Event {
	event := &Event{
		Metadata: make(Metadata),
		Data:     make([]byte, 0, len(e.Data)),
		Mimetype: e.Mimetype,
		Type:     e.Type,
		state:    initialized,
	}

	// Copy the metadata
	for key, val := range e.Metadata {
		event.Metadata[key] = val
	}

	// Copy the data
	copy(event.Data, e.Data)

	return event
}

// Compare two events to determine if they are equivalent by data.
// See Same() to determine if they are the same event by offset/topic.
func (e *Event) Equals(o *Event) bool {
	// Compare mimetype
	if e.Mimetype != o.Mimetype {
		return false
	}

	// Compare type
	if !e.Type.Equals(o.Type) {
		return false
	}

	// Compare created at timestamp
	if !e.Created.Equal(o.Created) {
		return false
	}

	// Compare metadata
	if len(e.Metadata) != len(o.Metadata) {
		return false
	}

	for key, val := range e.Metadata {
		if o.Metadata[key] != val {
			return false
		}
	}

	// Compare raw data payload
	return bytes.Equal(e.Data, o.Data)
}

// Convert an event into a protocol buffer event.
func (e *Event) toPB() *api.Event {
	return &api.Event{
		Data:     e.Data,
		Metadata: map[string]string(e.Metadata),
		Mimetype: e.Mimetype,
		Type:     e.Type,
		Created:  timestamppb.New(e.Created),
	}
}

// Returns the event wrapper which contains the API event info. Used for debugging.
func (e *Event) Info() *api.EventWrapper {
	return e.info
}

// Convert a protocol buffer event into this event.
func (e *Event) fromPB(wrapper *api.EventWrapper, state eventState) (_ <-chan *api.SubscribeRequest, err error) {
	if e.state != initialized {
		return nil, ErrOverwrite
	}

	// Set info on the wrapper
	e.info = wrapper

	var event *api.Event
	if event, err = wrapper.Unwrap(); err != nil {
		return nil, err
	}

	// Create channel for subscribe stream communication
	sub := make(chan *api.SubscribeRequest, 1)

	e.Data = event.Data
	e.Metadata = Metadata(event.Metadata)
	e.Mimetype = event.Mimetype
	e.Type = event.Type
	e.Created = event.Created.AsTime()
	e.state = state
	e.sub = sub

	return sub, nil
}
