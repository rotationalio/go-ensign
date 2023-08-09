package ensign

import (
	"context"
	"errors"
	"io"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

// QueryCursor exposes event results from an EnSQL query with familiar database cursor
// semantics.
type QueryCursor struct {
	stream api.Ensign_EnSQLClient
}

// read the next event from the stream and return it to the caller.
func (c *QueryCursor) read() (event *Event, err error) {
	if c.stream == nil {
		return nil, ErrCursorClosed
	}

	var wrapper *api.EventWrapper
	if wrapper, err = c.stream.Recv(); err != nil {
		// Return nil if the stream is closed
		if err == io.EOF {
			return nil, nil
		}

		return nil, err
	}

	// Convert the event into an API event
	event = &Event{}
	if err = event.fromPB(wrapper, query); err != nil {
		return nil, err
	}

	return event, nil
}

// FetchOne returns the next query result. If there are no more results then nil is
// returned.
func (i *QueryCursor) FetchOne() (*Event, error) {
	return i.read()
}

// FetchMany returns the next n query results. If there are less than n results
// remaining then all the remaining results are returned.
func (i *QueryCursor) FetchMany(n int) ([]*Event, error) {
	events := make([]*Event, 0, n)
	for len(events) < n {
		event, err := i.read()
		if err != nil {
			return nil, err
		}
		if event == nil {
			break
		}

		events = append(events, event)
	}
	return events, nil
}

// FetchAll returns all events from the query stream. If there are no more events then
// an empty slice is returned.
func (i *QueryCursor) FetchAll() ([]*Event, error) {
	events := make([]*Event, 0)
	for {
		event, err := i.read()
		if err != nil {
			return nil, err
		}
		if event == nil {
			break
		}

		events = append(events, event)
	}
	return events, nil
}

// Close the cursor, which closes the underlying stream.
func (i *QueryCursor) Close() (err error) {
	if err = i.stream.CloseSend(); err != nil {
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
	cursor = &QueryCursor{}
	if cursor.stream, err = c.api.EnSQL(ctx, query, c.copts...); err != nil {
		return nil, err
	}
	return cursor, nil
}

// Explain returns the query plan for the specified query.
func (c *Client) Explain(ctx context.Context, query *api.Query) (plan *api.QueryExplanation, err error) {
	return nil, errors.New("not implemented yet")
}
