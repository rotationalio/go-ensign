package topics_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	sdk "github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
	. "github.com/rotationalio/go-ensign/topics"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type topicTestSuite struct {
	suite.Suite
	mock  *mock.Ensign
	cache *Cache
}

func (s *topicTestSuite) SetupSuite() {
	assert := s.Assert()

	// Create a new mock ensign server for testing
	s.mock = mock.New(nil)

	// Create an sdk client that can be used as the topic client
	client, err := sdk.New(
		sdk.WithMock(s.mock),
		sdk.WithAuthenticator("", true),
	)
	assert.NoError(err, "could not connect ensign client to mock")

	// Create the cache for testing
	s.cache = NewCache(client)
}

func (s *topicTestSuite) AfterTest(suiteName, testName string) {
	// Cleanup the mock and the cache after testing.
	s.mock.Reset()
	s.cache.Clear()
}

func TestTopicsSuite(t *testing.T) {
	suite.Run(t, &topicTestSuite{})
}

func (s *topicTestSuite) TestGet() {
	// The topic cache should be empty to start and make a request to Ensign; after
	// which point the topic name should be retrieved from the cache without an RPC.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	// Have list topics return a list of topic names to search for the topicID
	err := s.mock.UseFixture(mock.TopicNamesRPC, "testdata/topicnames.pb.json")
	require.NoError(err, "could not load topic names fixture")

	// The first lookup should make a request to the ensign mock
	// Subsequent lookups should simply use the cache
	for i := 0; i < 10; i++ {
		topicID, err := s.cache.Get("testing.topics.topicb")
		require.NoError(err, "could not lookup topic id")
		require.Equal("01GWM936SNSN36JKTMSF9Q3N8B", topicID, "unexpected topicId returned")
	}

	require.Equal(1, s.cache.Length(), "expected cache to only have one item")
	require.Equal(1, s.mock.Calls[mock.TopicNamesRPC], "expected the RPC to be called only once")
	require.Len(s.mock.Calls, 1, "expected only one RPC called")

	// A lookup to another topic should not cause the cache to fail
	for i := 0; i < 10; i++ {
		topicID, err := s.cache.Get("testing.topics.topica")
		require.NoError(err, "could not lookup topic id")
		require.Equal("01GWM89049D49FHJH81BT8795H", topicID, "unexpected topicId returned")
	}

	topicID, err := s.cache.Get("testing.topics.topicb")
	require.NoError(err, "could not lookup topic id")
	require.Equal("01GWM936SNSN36JKTMSF9Q3N8B", topicID, "unexpected topicId returned")

	require.Equal(2, s.cache.Length(), "expected cache to only have one item")
	require.Equal(2, s.mock.Calls[mock.TopicNamesRPC], "expected the RPC to be called only once")
	require.Len(s.mock.Calls, 1, "expected only one RPC called")
}

func (s *topicTestSuite) TestGetFail() {
	// Test errors returned from topic Get
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	// Test error returned from server
	s.mock.UseError(mock.TopicNamesRPC, codes.DataLoss, "something bad happened")
	_, err := s.cache.Get("testing.topics.topicc")
	require.EqualError(err, "rpc error: code = DataLoss desc = something bad happened", "expected the error from the ensign server")

	// Test not found in fixtures
	err = s.mock.UseFixture(mock.TopicNamesRPC, "testdata/topicnames.pb.json")
	require.NoError(err, "could not load topic names fixture")

	_, err = s.cache.Get("testing.topics.does-not-exist")
	require.ErrorIs(err, ErrTopicNotFound)

	require.Equal(0, s.cache.Length(), "expected cache to be empty")
	require.Equal(2, s.mock.Calls[mock.TopicNamesRPC], "expected the RPC to be called only once")
	require.Len(s.mock.Calls, 1, "expected only one RPC called")
}

func (s *topicTestSuite) TestExists() {
	// Test the topic existence check functionality.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	existingTopics := map[string]struct{}{
		"testing.topics.topica": {},
		"testing.topics.topicb": {},
		"testing.topics.topicc": {},
	}
	s.mock.OnTopicExists = func(ctx context.Context, in *api.TopicName) (out *api.TopicExistsInfo, err error) {
		out = &api.TopicExistsInfo{
			Query: fmt.Sprintf("name=%q", in.Name),
		}

		_, out.Exists = existingTopics[in.Name]
		return out, nil
	}

	for i := 0; i < 10; i++ {
		exists, err := s.cache.Exists("testing.topics.topicb")
		require.NoError(err, "could not call topic exists")
		require.True(exists, "the topic should exist")

		exists, err = s.cache.Exists("foo.bar.notreal")
		require.NoError(err, "could not call topic exists")
		require.False(exists, "the topic should not exist")
	}

	require.Equal(0, s.cache.Length(), "expected cache to be empty; nothing to cache on existence")
	require.Equal(20, s.mock.Calls[mock.TopicExistsRPC], "expected the RPC to be called 20 times, for each existence check")
	require.Len(s.mock.Calls, 1, "expected only one RPC called")
}

func (s *topicTestSuite) TestExistsCache() {
	// Topic exists should return true if the topic is in the cache.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	// Have list topics return a list of topic names to search for the topicID
	err := s.mock.UseFixture(mock.TopicNamesRPC, "testdata/topicnames.pb.json")
	require.NoError(err, "could not load topic names fixture")

	s.cache.Get("testing.topics.topica")

	exists, err := s.cache.Exists("testing.topics.topica")
	require.NoError(err, "could not check existence of topics")
	require.True(exists, "topic should exist in cache")

	require.Equal(1, s.cache.Length(), "expected cache to contain one item")
	require.Equal(1, s.mock.Calls[mock.TopicNamesRPC], "expected the topic names RPC to be called only once")
	require.Equal(0, s.mock.Calls[mock.TopicExistsRPC], "expected the topic exists RPC to not be called")
	require.Len(s.mock.Calls, 1, "expected only one RPC called")
}

func (s *topicTestSuite) TestExistsError() {
	// An error should be returned if the RPC errors
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	s.mock.UseError(mock.TopicExistsRPC, codes.Internal, "something bad happened")

	_, err := s.cache.Exists("testing.topics.topica")
	require.EqualError(err, "rpc error: code = Internal desc = something bad happened")
}

func (s *topicTestSuite) TestEnsure() {
	// The topic cache should be empty to start but populated with topic name to IDs to
	// prevent multiple Ensign RPC calls. If the topic doesn't exist, it should be
	// created.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	existingTopics := map[string]struct{}{
		"testing.topics.topica": {},
		"testing.topics.topicb": {},
		"testing.topics.topicc": {},
	}
	s.mock.OnTopicExists = func(ctx context.Context, in *api.TopicName) (out *api.TopicExistsInfo, err error) {
		out = &api.TopicExistsInfo{
			Query: fmt.Sprintf("name=%q", in.Name),
		}

		_, out.Exists = existingTopics[in.Name]
		return out, nil
	}

	// Have list topics return a list of topic names to search for the topicID
	err := s.mock.UseFixture(mock.TopicNamesRPC, "testdata/topicnames.pb.json")
	require.NoError(err, "could not load topic names fixture")

	// Have create topic return a unique topic
	s.mock.OnCreateTopic = func(ctx context.Context, in *api.Topic) (*api.Topic, error) {
		in.Id = ulid.Make().Bytes()
		return in, nil
	}

	// The first lookup should make a request to the ensign mock
	// Subsequent lookups should simply use the cache
	for i := 0; i < 10; i++ {
		topicID, err := s.cache.Ensure("testing.topics.topicb")
		require.NoError(err, "could not lookup topic id")
		require.Equal("01GWM936SNSN36JKTMSF9Q3N8B", topicID, "unexpected topicId returned")
	}

	require.Equal(1, s.cache.Length(), "expected cache to only have one item")
	require.Equal(1, s.mock.Calls[mock.TopicNamesRPC], "expected the topic names RPC to be called only once")
	require.Equal(1, s.mock.Calls[mock.TopicExistsRPC], "expected the topic exists RPC to be called only once")
	require.Equal(0, s.mock.Calls[mock.CreateTopicRPC], "expected the create topic RPC to be called zero times")
	require.Len(s.mock.Calls, 2, "expected two RPCs called")

	// A lookup to a topic that does not exist should create the topic
	for i := 0; i < 10; i++ {
		topicID, err := s.cache.Ensure("testing.topics.foo")
		require.NoError(err, "could not lookup topic id")
		require.NotZero(topicID, "expected a ULID topic id to be returned")
	}

	require.Equal(2, s.cache.Length(), "expected cache to have two items")
	require.Equal(1, s.mock.Calls[mock.TopicNamesRPC], "expected the topic names RPC to be called only once")
	require.Equal(2, s.mock.Calls[mock.TopicExistsRPC], "expected the topic exists RPC to be called twice")
	require.Equal(1, s.mock.Calls[mock.CreateTopicRPC], "expected the create topic RPC to be called zero times")
	require.Len(s.mock.Calls, 3, "expected thre RPCs called")
}

func (s *topicTestSuite) TestEnsureCreateError() {
	// The topic cache should be empty to start and make a request to Ensign; after
	// which point the topic name should be retrieved from the cache without an RPC.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	s.mock.OnTopicExists = func(context.Context, *api.TopicName) (*api.TopicExistsInfo, error) {
		return &api.TopicExistsInfo{
			Exists: false,
		}, nil
	}

	s.mock.UseError(mock.CreateTopicRPC, codes.Internal, "couldn't create topic")

	_, err := s.cache.Ensure("testing.topics.topica")
	require.EqualError(err, "rpc error: code = Internal desc = couldn't create topic")
}

func (s *topicTestSuite) TestEnsureExistsError() {
	// The topic cache should be empty to start and make a request to Ensign; after
	// which point the topic name should be retrieved from the cache without an RPC.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	s.mock.UseError(mock.TopicExistsRPC, codes.Internal, "topic hard to check")

	_, err := s.cache.Ensure("testing.topics.topica")
	require.EqualError(err, "rpc error: code = Internal desc = topic hard to check")
}

func (s *topicTestSuite) TestEnsureTopicIDError() {
	// The topic cache should be empty to start and make a request to Ensign; after
	// which point the topic name should be retrieved from the cache without an RPC.
	require := s.Require()
	require.Equal(0, s.cache.Length(), "expected cache to be empty")

	s.mock.OnTopicExists = func(context.Context, *api.TopicName) (*api.TopicExistsInfo, error) {
		return &api.TopicExistsInfo{
			Exists: true,
		}, nil
	}

	s.mock.UseError(mock.TopicNamesRPC, codes.Internal, "couldn't get topic id")

	_, err := s.cache.Ensure("testing.topics.topica")
	require.EqualError(err, "rpc error: code = Internal desc = couldn't get topic id")
}
