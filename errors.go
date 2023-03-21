package sdk

import "errors"

var (
	ErrMissingEndpoint     = errors.New("invalid options: endpoint is required")
	ErrMissingClientID     = errors.New("invalid options: client ID is required")
	ErrMissingClientSecret = errors.New("invalid options: client secret is required")
	ErrMissingAuthURL      = errors.New("invalid options: auth url is required")
	ErrTopicNameNotFound   = errors.New("topic name not found in project")
)

type Errorer interface {
	Err() error
}
