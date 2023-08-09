package ensign_test

import (
	"context"

	"github.com/oklog/ulid/v2"
	"github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *sdkTestSuite) TestEnSQL() {
	require := s.Require()
	ctx := context.Background()

	require.NoError(s.Authenticate(ctx))

	// Create the fixture events
	events := []*api.Event{
		{
			Data: []byte(`{"name": "Alice"}`),
			Metadata: map[string]string{
				"foo": "bar",
			},
			Mimetype: mimetype.ApplicationJSON,
			Type: &api.Type{
				Name:         "Person",
				MajorVersion: 1,
			},
			Created: timestamppb.Now(),
		},
		{
			Data: []byte("hello world"),
			Metadata: map[string]string{
				"bar": "baz",
			},
			Mimetype: mimetype.TextPlain,
			Type: &api.Type{
				Name:         "Message",
				MajorVersion: 1,
			},
			Created: timestamppb.Now(),
		},
		{
			Data: []byte(`{"name": "Bob"}`),
			Type: &api.Type{
				Name:         "Person",
				MajorVersion: 2,
				MinorVersion: 1,
			},
			Created: timestamppb.Now(),
		},
	}

	// Setup the mock to return the fixture events
	topicID := ulid.MustParse("01GZ1ASDEPPFWD485HSQKDAS4K")
	s.mock.OnEnSQL = func(in *api.Query, stream api.Ensign_EnSQLServer) (err error) {
		for _, event := range events {
			// Wrap the event in the wrapper
			wrapper := &api.EventWrapper{
				TopicId:   topicID[:],
				Committed: timestamppb.Now(),
			}

			if err = wrapper.Wrap(event); err != nil {
				return err
			}

			if err := stream.Send(wrapper); err != nil {
				return err
			}
		}
		return nil
	}

	// Test an error is returned if the query is empty
	_, err := s.client.EnSQL(context.Background(), &api.Query{})
	require.ErrorIs(err, ensign.ErrEmptyQuery, "expected error for empty query")

	// Test with a valid query
	query := &api.Query{
		Query: "SELECT * FROM topic",
	}
	cursor, err := s.client.EnSQL(context.Background(), query)
	require.NoError(err, "expected no error for valid query")

	// Fetch all the events at once
	results, err := cursor.FetchAll()
	require.NoError(err, "expected no error fetching all events")
	require.Len(results, len(events), "expected all events to be returned")
	_, err = cursor.FetchAll()
	require.ErrorIs(err, ensign.ErrCursorClosed, "expected cursor to be closed")

	// Fetch multiple events
	cursor, err = s.client.EnSQL(context.Background(), query)
	require.NoError(err, "expected no error for valid query")
	results, err = cursor.FetchMany(2)
	require.NoError(err, "expected no error fetching multiple events")
	require.Len(results, 2, "expected two events to be returned")
	results, err = cursor.FetchMany(2)
	require.NoError(err, "expected no error fetching multiple events again")
	require.Len(results, 1, "expected one event to be returned")
	_, err = cursor.FetchMany(2)
	require.ErrorIs(err, ensign.ErrCursorClosed, "expected cursor to be closed")

	// Fetch one at a time
	cursor, err = s.client.EnSQL(context.Background(), query)
	require.NoError(err, "expected no error for valid query")
	for i := 0; i < len(events); i++ {
		event, err := cursor.FetchOne()
		require.NoError(err, "expected no error fetching one event")
		require.NotNil(event, "expected event to be returned")
		require.Equal(events[i].Data, event.Data, "expected event data to match")

		// Cannot ack or nack a query result
		_, err = event.Ack()
		require.ErrorIs(err, ensign.ErrCannotAck, "expected error; cannot ack query result")
		_, err = event.Nack(api.Nack_UNKNOWN_TYPE)
		require.ErrorIs(err, ensign.ErrCannotAck, "expected error; cannot nack query result")
	}

	// Cursor is now at the end, next event should be nil
	event, err := cursor.FetchOne()
	require.NoError(err, "expected no error when no more results")
	require.Nil(event, "expected no more events to be returned")
	_, err = cursor.FetchOne()
	require.ErrorIs(err, ensign.ErrCursorClosed, "expected cursor to be closed")

	// After close the cursor returns an error
	cursor, err = s.client.EnSQL(context.Background(), query)
	require.NoError(cursor.Close(), "expected no error closing cursor")
	_, err = cursor.FetchOne()
	require.ErrorIs(err, ensign.ErrCursorClosed, "expected error fetching one event after close")

	// Test that an error is returned if the server returns an error
	// TODO: The mock server is not returning an error, not sure why
	require.NoError(s.mock.UseError(mock.EnSQLRPC, codes.InvalidArgument, "unparseable query"))
	_, err = s.client.EnSQL(ctx, query)
	s.GRPCErrorIs(err, codes.InvalidArgument, "unparseable query")
}
