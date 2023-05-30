package stream_test

import (
	"errors"
	"io"
	"testing"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/stream"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type subscriberTestSuite struct {
	suite.Suite
	mock *MockConnectionObserver
}

// Create the bufconn and mock when the suite starts.
func (s *subscriberTestSuite) SetupSuite() {
	var err error
	s.mock, err = NewMockConnectionObserver()
	s.Assert().NoError(err, "unable to setup mock suite")
}

// When the suite is done teardown the bufconn and mock connections.
func (s *subscriberTestSuite) TearDownSuite() {
	s.mock.conn.Close()
	s.mock.server.Shutdown()
	s.mock.sock.Close()
}

// After each test make sure the mock server is reset.
func (s *subscriberTestSuite) AfterTest(suiteName, testName string) {
	s.mock.server.Reset()
}

func TestSubscriber(t *testing.T) {
	suite.Run(t, &subscriberTestSuite{})
}

func (s *subscriberTestSuite) TestSubscriberTopics() {
	// When the stream is opened, send a topic map back.
	s.mock.server.OnSubscribe = func(stream api.Ensign_SubscribeServer) error {
		// Get the open stream message
		msg, err := stream.Recv()
		if err != nil {
			return status.Error(codes.Aborted, "stream canceled before initialization")
		}

		switch t := msg.Embed.(type) {
		case *api.SubscribeRequest_Subscription:
			if len(t.Subscription.Topics) != 2 {
				return status.Error(codes.InvalidArgument, "bad topics sent")
			}

			if err := stream.Send(&api.SubscribeReply{
				Embed: &api.SubscribeReply_Ready{
					Ready: &api.StreamReady{
						ClientId: t.Subscription.ClientId,
						ServerId: "mock",
						Topics: map[string][]byte{
							"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ").Bytes(),
							"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS").Bytes(),
						},
					},
				},
			}); err != nil {
				return status.Error(codes.Canceled, "stream canceled before topic map sent back")
			}

		default:
			return status.Error(codes.FailedPrecondition, "must send a subscription message first")
		}

		// Block until the stream is closed by the client.
		if _, err := stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return status.Error(codes.Aborted, "stream closed with error")
		}
		return nil
	}

	// Create the subscriber
	require := s.Require()
	_, sub, err := stream.NewSubscriber(s.mock, []string{"testing.123", "example.456"})
	require.NoError(err, "could not connect to subscriber")
	require.NoError(sub.Err(), "subscriber has an error attached")

	topics := sub.Topics()
	require.Len(topics, 2)
	require.Contains(topics, "testing.123")
	require.Equal(topics["testing.123"], ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"))
	require.Contains(topics, "example.456")
	require.Equal(topics["example.456"], ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"))

	require.NoError(sub.Close())
}
