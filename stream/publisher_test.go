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

type publisherTestSuite struct {
	suite.Suite
	mock *MockConnectionObserver
}

// Create the bufconn and mock when the suite starts.
func (s *publisherTestSuite) SetupSuite() {
	var err error
	s.mock, err = NewMockConnectionObserver()
	s.Assert().NoError(err, "unable to setup mock suite")
}

// When the suite is done teardown the bufconn and mock connections.
func (s *publisherTestSuite) TearDownSuite() {
	s.mock.conn.Close()
	s.mock.server.Shutdown()
	s.mock.sock.Close()
}

// After each test make sure the mock server is reset.
func (s *publisherTestSuite) AfterTest(suiteName, testName string) {
	s.mock.server.Reset()
}

func TestPublisher(t *testing.T) {
	suite.Run(t, &publisherTestSuite{})
}

func (s *publisherTestSuite) TestPublisherTopics() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	handler := mock.NewPublishHandler(fixture)
	s.mock.server.OnPublish = handler.OnPublish

	// Create the publisher
	require := s.Require()
	pub, err := stream.NewPublisher(s.mock)
	require.NoError(err, "could not connect to publisher")
	require.NoError(pub.Err(), "publisher has an error atached")

	topics := pub.Topics()
	require.Equal(fixture, topics)
	require.NoError(pub.Close())
}

func (s *publisherTestSuite) TestPublisherNotAuthorized() {
	handler := mock.NewPublishHandler(nil)
	handler.OnInitialize = func(*api.OpenStream) (*api.StreamReady, error) {
		return nil, status.Error(codes.Unauthenticated, "bad api keys")
	}
	s.mock.server.OnPublish = handler.OnPublish

	require := s.Require()
	pub, err := stream.NewPublisher(s.mock)
	require.Nil(pub)
	CheckStatusError(require, err, codes.Unauthenticated, "bad api keys")
}

func (s *publisherTestSuite) TestPublisherTopicNames() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	handler := mock.NewPublishHandler(fixture)
	s.mock.server.OnPublish = handler.OnPublish

	// Create the publisher
	require := s.Require()
	pub, err := stream.NewPublisher(s.mock)
	require.NoError(err, "could not connect to publisher")

	for i := 0; i < 10; i++ {
		var topic string
		if i < 5 {
			topic = "testing.123"
		} else {
			topic = "example.456"
		}

		event := mock.NewEvent()
		C, err := pub.Publish(topic, event)
		require.NoError(err, "could not publish event with topic name")
		rep := <-C
		ack := rep.GetAck()
		require.NotNil(ack)
		require.NotEmpty(ack.Id)
	}

	require.NoError(pub.Close())
}

func (s *publisherTestSuite) TestCannotResolveTopicID() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	handler := mock.NewPublishHandler(fixture)
	handler.OnEvent = func(in *api.EventWrapper) (out *api.PublisherReply, err error) {
		topicID := ulid.ULID{}
		if err = topicID.UnmarshalBinary(in.TopicId); err != nil {
			return nil, status.Error(codes.InvalidArgument, "bad topic ID")
		}

		found := false
		for _, validID := range fixture {
			if validID.Compare(topicID) == 0 {
				found = true
				break
			}
		}

		out = &api.PublisherReply{}
		if !found {
			out.Embed = &api.PublisherReply_Nack{
				Nack: &api.Nack{
					Id:   in.LocalId,
					Code: api.Nack_TOPIC_UKNOWN,
				},
			}
		} else {
			out.Embed = &api.PublisherReply_Ack{
				Ack: &api.Ack{
					Id: in.LocalId,
				},
			}
		}

		return out, nil
	}
	s.mock.server.OnPublish = handler.OnPublish

	// Create the publisher
	require := s.Require()
	pub, err := stream.NewPublisher(s.mock)
	require.NoError(err, "could not connect to publisher")

	// Could not resolve topic name
	C, err := pub.Publish("notatopic", mock.NewEvent())
	require.Nil(C)
	require.ErrorIs(err, stream.ErrResolveTopic)

	// Nack ULID
	C, err = pub.Publish(ulid.Make().String(), mock.NewEvent())
	require.NoError(err, "expected to be able to publish any ulid")
	rep := <-C
	nack := rep.GetNack()
	require.NotNil(nack, "expected a nack")
	require.Equal(api.Nack_TOPIC_UKNOWN, nack.Code)

	require.NoError(pub.Close())
}

func (s *publisherTestSuite) TestPublisherTopicIDs() {
	// When the stream is opened, send a topic map back.
	fixture := map[string]ulid.ULID{
		"testing.123": ulid.MustParse("01H1PA4FA9G2Y79Z5FC36CWYYJ"),
		"example.456": ulid.MustParse("01H1PA4P7C6VT5KZCXH56H1XHS"),
	}

	handler := mock.NewPublishHandler(fixture)
	s.mock.server.OnPublish = handler.OnPublish

	// Create the publisher
	require := s.Require()
	pub, err := stream.NewPublisher(s.mock)
	require.NoError(err, "could not connect to publisher")

	for i := 0; i < 10; i++ {
		var topic string
		if i < 5 {
			topic = "01H1PA4FA9G2Y79Z5FC36CWYYJ"
		} else {
			topic = "01H1PA4P7C6VT5KZCXH56H1XHS"
		}

		event := mock.NewEvent()
		C, err := pub.Publish(topic, event)
		require.NoError(err, "could not publish event with topic ID")
		rep := <-C
		ack := rep.GetAck()
		require.NotNil(ack)
		require.NotEmpty(ack.Id)
	}

	require.NoError(pub.Close())
}

func (s *publisherTestSuite) TestPublisherReconnect() {
	s.T().Skip("TODO: implement publisher reconnect test")
}
