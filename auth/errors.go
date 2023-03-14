package auth

import (
	"fmt"
	"net/http"
)

var (
	unsuccessful = Reply{Success: false}
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

// Reply contains standard fields that are used for generic API responses and errors.
type Reply struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
