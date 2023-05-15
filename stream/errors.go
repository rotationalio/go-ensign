package stream

import "errors"

type Errorer interface {
	Err() error
}

var (
	ErrStreamUninitialized = errors.New("could not initialize stream with server")
)
