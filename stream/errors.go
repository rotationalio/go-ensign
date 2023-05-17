package stream

import "errors"

type Errorer interface {
	Err() error
}

var (
	ErrStreamUninitialized = errors.New("could not initialize stream with server")
	ErrReconnect           = errors.New("failed to reconnect to remote server within timeout")
	ErrResolveTopic        = errors.New("could not resolve topic, specify topic ID or allowed topic name")
)
