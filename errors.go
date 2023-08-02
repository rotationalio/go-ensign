package ensign

import (
	"errors"
	"fmt"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

// Standardized errors that the client may return from configuration issues or parsed
// from gRPC service calls. These errors can be evaluated using errors.Is to test for
// different error conditions in client code.
var (
	ErrMissingEndpoint     = errors.New("invalid options: endpoint is required")
	ErrMissingClientID     = errors.New("invalid options: client ID is required")
	ErrMissingClientSecret = errors.New("invalid options: client secret is required")
	ErrMissingAuthURL      = errors.New("invalid options: auth url is required")
	ErrMissingMock         = errors.New("invalid options: in testing mode a mock grpc server is required")
	ErrTopicNameNotFound   = errors.New("topic name not found in project")
	ErrCannotAck           = errors.New("cannot ack or nack an event not received from subscribe")
	ErrOverwrite           = errors.New("this operation would overwrite existing event data")
	ErrNoTopicID           = errors.New("topic id is not available on event")
)

// A Nack from the server on a publish stream indicates that the event was not
// successfully published for the reason specified by the code and the message. Nacks
// received by the publisher indicate that the event should be retried or dropped.
// Subscribers can also send NackErrors to the Ensign server in order to indicate that
// the message be replayed to a different client or that the consumer group offset
// should not be updated since the event was unhandled.
type NackError struct {
	ID      []byte
	Code    api.Nack_Code
	Message string
}

// Error implements the error interface so that a NackError can be returned as an error.
func (e *NackError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s", e.Code.String(), e.Message)
	}
	return e.Code.String()
}

func makeNackError(nack *api.Nack) error {
	return &NackError{
		ID:      nack.Id,
		Code:    nack.Code,
		Message: nack.Error,
	}
}
