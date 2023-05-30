package stream_test

import (
	"testing"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
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
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	// Setup the server mock with a subscribe handler that uses the topics fixture
	handler := mock.NewSubscribeHandler()
	handler.UseTopicMap(fixture)
	s.mock.server.OnSubscribe = handler.OnSubscribe
	defer handler.Shutdown()

	// Create the subscriber
	require := s.Require()
	_, sub, err := stream.NewSubscriber(s.mock, []string{"testing.123", "example.456"})
	require.NoError(err, "could not connect to subscriber")
	require.NoError(sub.Err(), "subscriber has an error attached")

	topics := sub.Topics()
	require.Equal(fixture, topics)
	require.NoError(sub.Close())
}

func (s *subscriberTestSuite) TestSubscriberBadSubscription() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	// Setup the server mock with a subscribe handler that uses the topics fixture
	handler := mock.NewSubscribeHandler()
	handler.UseTopicMap(fixture)
	s.mock.server.OnSubscribe = handler.OnSubscribe
	defer handler.Shutdown()

	require := s.Require()
	_, _, err := stream.NewSubscriber(s.mock, []string{"testing.123", "badtopic.789"})
	CheckStatusError(require, err, codes.InvalidArgument, "unknown topic \"badtopic.789\"")
}

func (s *subscriberTestSuite) TestSubscriberNotAuthorized() {
	// Setup the server mock with a subscribe handler that uses the topics fixture
	handler := mock.NewSubscribeHandler()
	handler.OnInitialize = func(*api.Subscription) (*api.StreamReady, error) {
		return nil, status.Error(codes.Unauthenticated, "bad api keys")
	}
	s.mock.server.OnSubscribe = handler.OnSubscribe
	defer handler.Shutdown()

	require := s.Require()
	_, _, err := stream.NewSubscriber(s.mock, nil)
	CheckStatusError(require, err, codes.Unauthenticated, "bad api keys")
}

func (s *subscriberTestSuite) TestSubscriberFixedEvents() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	// Setup the server mock with a subscribe handler that uses the topics fixture
	handler := mock.NewSubscribeHandler()
	handler.UseTopicMap(fixture)
	s.mock.server.OnSubscribe = handler.OnSubscribe
	defer handler.Shutdown()

	require := s.Require()
	C, sub, err := stream.NewSubscriber(s.mock, nil)
	require.NoError(err, "could not open subscriber")

	// Send and recv events (expect that the send buffer is 64)
	for i := 0; i < 10; i++ {
		handler.Send <- mock.NewEvent()
		evt := <-C
		require.NoError(sub.Ack(&api.Ack{Id: evt.Id}))
	}

	require.NoError(sub.Close())
	require.NoError(sub.Err())
}
