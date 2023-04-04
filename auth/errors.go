package auth

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrIncompleteCreds = errors.New("both client id and secret are required")
	ErrNoAPIKeys       = errors.New("no api keys available: must login the client first")
	unsuccessful       = Reply{Success: false}
)

// StatusError decodes an error response from Quarterdeck.
type StatusError struct {
	StatusCode int
	Reply      Reply
}

func (e *StatusError) Error() string {
	if e.Reply.Error != "" {
		return fmt.Sprintf("[%d] %s", e.StatusCode, e.Reply.Error)
	}
	return fmt.Sprintf("[%d] %s", e.StatusCode, http.StatusText(e.StatusCode))
}
