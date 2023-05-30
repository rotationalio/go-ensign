package stream_test

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/rotationalio/go-ensign/mock"
	"github.com/rotationalio/go-ensign/stream"
	"github.com/stretchr/testify/suite"
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
	require.Len(topics, 2)
	require.Contains(topics, "testing.123")
	require.Equal(topics["testing.123"], ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"))
	require.Contains(topics, "example.456")
	require.Equal(topics["example.456"], ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"))

	require.NoError(sub.Close())
}
