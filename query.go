package ensign

import (
	"context"
	"io"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QueryCursor exposes event results from an EnSQL query with familiar database cursor
// semantics. Note that the cursor is not thread safe and should only be used from a
// single thread.
type QueryCursor struct {
	stream api.Ensign_EnSQLClient
	result *Event
}

// NewQueryCursor creates a new query cursor that reads from the specified stream.
func NewQueryCursor(stream api.Ensign_EnSQLClient) (cursor *QueryCursor, err error) {
	cursor = &QueryCursor{
		stream: stream,
	}

	// Fetch the first event to catch any errors.
	if cursor.result, err = cursor.FetchOne(); err != nil {
		return nil, err
	}

	return cursor, nil
}

// read fetches the next event from the stream and returns the previous result to the
// caller. If there are no more events then a nil event is returned.
func (c *QueryCursor) read() (event *Event, err error) {
	if c.stream == nil {
		return nil, ErrCursorClosed
	}

	// If there's a cached result then return it
	if c.result != nil {
		event = c.result
		c.result = nil
		return event, nil
	}

	// Read the next event and cache it
	var wrapper *api.EventWrapper
	if wrapper, err = c.stream.Recv(); err != nil {
		if streamClosed(err) {
			c.Close()
			return nil, nil
		}
		return nil, err
	}

	// Convert the event into an API event
	event = &Event{}
	if err = event.fromPB(wrapper, query); err != nil {
		c.Close()
		return nil, err
	}

	return event, nil
}

// FetchOne returns the next query result. If there are no more results then nil is
// returned.
func (i *QueryCursor) FetchOne() (event *Event, err error) {
	if event, err = i.read(); err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrNoRows
	}
	return event, nil
}

// FetchMany returns the next n query results. If there are less than n results
// remaining then all the remaining results are returned.
func (i *QueryCursor) FetchMany(n int) (events []*Event, err error) {
	events = make([]*Event, 0, n)
	for len(events) < n {
		var event *Event
		if event, err = i.read(); err != nil {
			return nil, err
		}

		if event == nil {
			break
		}

		events = append(events, event)
	}

	if len(events) == 0 {
		return nil, ErrNoRows
	}

	return events, nil
}

// FetchAll returns all events from the query stream. If there are no more events then
// an empty slice is returned.
func (i *QueryCursor) FetchAll() (events []*Event, err error) {
	events = make([]*Event, 0)
	for {
		var event *Event
		if event, err = i.read(); err != nil {
			return nil, err
		}

		if event == nil {
			break
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		return nil, ErrNoRows
	}

	return events, nil
}

// Close the cursor, which closes the underlying stream.
func (i *QueryCursor) Close() (err error) {
	if i.stream == nil {
		return nil
	}

	if err = i.stream.CloseSend(); err != nil {
		i.stream = nil
		return err
	}

	i.stream = nil
	return nil
}

// EnSQL executes a query against Ensign and returns a cursor that can be used to fetch
// the event results. This RPC always returns a finite number of results. After all
// results have been returned the cursor will return nil. In order to retrieve events
// in a more streaming fashion, the Subscribe RPC should be used with a query option.
func (c *Client) EnSQL(ctx context.Context, query *api.Query) (cursor *QueryCursor, err error) {
	if query.Query == "" {
		return nil, ErrEmptyQuery
	}

	// Create the stream by sending the query request to the server.
	var stream api.Ensign_EnSQLClient
	if stream, err = c.api.EnSQL(ctx, query, c.copts...); err != nil {
		return nil, err
	}

	return NewQueryCursor(stream)
}

// Explain returns the query plan for the specified query, including the expected
// number of results and errors that might be returned.
func (c *Client) Explain(ctx context.Context, query *api.Query) (plan *api.QueryExplanation, err error) {
	if query.Query == "" {
		return nil, ErrEmptyQuery
	}

	return c.api.Explain(ctx, query, c.copts...)
}

func streamClosed(err error) bool {
	if err == io.EOF {
		return true
	}

	if serr, ok := status.FromError(err); ok {
		if serr.Code() == codes.Canceled {
			return true
		}
	}

	return false
}
