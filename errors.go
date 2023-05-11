package ensign

import (
	"errors"
	"fmt"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

var (
	ErrMissingEndpoint     = errors.New("invalid options: endpoint is required")
	ErrMissingClientID     = errors.New("invalid options: client ID is required")
	ErrMissingClientSecret = errors.New("invalid options: client secret is required")
	ErrMissingAuthURL      = errors.New("invalid options: auth url is required")
	ErrMissingMock         = errors.New("invalid options: in testing mode a mock grpc server is required")
	ErrTopicNameNotFound   = errors.New("topic name not found in project")
	ErrStreamUninitialized = errors.New("could not initialize stream with server")
	ErrCannotAck           = errors.New("cannot ack or nack an event not received from subscribe")
	ErrOverwrite           = errors.New("this operation would overwrite existing event data")
)

type Errorer interface {
	Err() error
}

type NackError struct {
	ID      []byte
	Code    api.Nack_Code
	Message string
}

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
