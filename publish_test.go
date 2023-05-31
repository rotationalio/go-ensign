package ensign_test

import (
	"context"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
)

func (s *sdkTestSuite) TestPublish() {
	s.Authenticate(context.Background())
	handler := mock.NewPublishHandler(nil)
	s.mock.OnPublish = handler.OnPublish

	require := s.Require()

	// Test publish single event
	event := NewEvent()
	err := s.client.Publish("01H1S1F67V282KQJSWAMARG8QF", event)
	require.NoError(err, "could not publish single event")

	acked, err := event.Acked()
	require.True(acked, "expected event to be acked")
	require.NoError(err)
	require.NoError(event.Err())
}

func (s *sdkTestSuite) TestPublishStream() {
	// This is mostly a sanity check to make sure the mock is working.
	s.Authenticate(context.Background())
	handler := mock.NewPublishHandler(nil)
	s.mock.OnPublish = handler.OnPublish

	require := s.Require()

	stream, err := s.client.PublishStream(context.Background())
	require.NoError(err)
	err = stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_OpenStream{OpenStream: &api.OpenStream{ClientId: "foo"}}})
	require.NoError(err)

	msg, err := stream.Recv()
	require.NoError(err)
	require.NotNil(msg.GetReady(), "expected a ready reply")

	// Send events
	err = stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_Event{Event: mock.NewEventWrapper()}})
	require.NoError(err)

	msg, err = stream.Recv()
	require.NoError(err)
	require.NotNil(msg.GetAck(), "expected an ack from the server")
}
