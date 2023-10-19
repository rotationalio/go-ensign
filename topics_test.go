package ensign_test

import (
	"context"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *sdkTestSuite) TestSetTopicDeduplicationPolicy() {
	require := s.Require()
	topicID := "01HCG64Y1SMFQBW7A42SRV207A"
	names := []string{"alpha", "bravo", "charlie"}

	// Authenticate the client
	ctx := context.Background()
	require.NoError(s.Authenticate(ctx))

	s.Run("UnsupportedKeys", func() {
		policies := []api.Deduplication_Strategy{
			api.Deduplication_NONE,
			api.Deduplication_STRICT,
			api.Deduplication_DATAGRAM,
		}

		for _, policy := range policies {
			_, err := s.client.SetTopicDeduplicationPolicy(ctx, topicID, policy, api.Deduplication_OFFSET_EARLIEST, names, false)
			require.Error(err, "expected keys unsupported error for %s policy", policy.String())
		}
	})

	s.Run("APIError", func() {
		defer s.mock.Reset()
		s.mock.UseError(mock.SetTopicPolicyRPC, codes.FailedPrecondition, "mock error")
		_, err := s.client.SetTopicDeduplicationPolicy(ctx, topicID, api.Deduplication_DATAGRAM, api.Deduplication_OFFSET_EARLIEST, nil, false)
		s.GRPCErrorIs(err, codes.FailedPrecondition, "mock error")
	})

	s.Run("NoKeysOrFields", func() {
		defer s.mock.Reset()
		s.mock.OnSetTopicPolicy = func(ctx context.Context, tp *api.TopicPolicy) (*api.TopicStatus, error) {
			if tp.Id != topicID {
				return nil, status.Error(codes.InvalidArgument, "incorrect topic id")
			}

			if tp.DeduplicationPolicy == nil || tp.ShardingStrategy != api.ShardingStrategy_UNKNOWN {
				return nil, status.Error(codes.InvalidArgument, "missing deduplication policy or sharding strategy set")
			}

			policy := tp.DeduplicationPolicy
			if policy.Strategy != api.Deduplication_DATAGRAM || policy.Offset != api.Deduplication_OFFSET_LATEST {
				return nil, status.Error(codes.InvalidArgument, "deduplication policy not set correctly")
			}

			if len(policy.Keys) != 0 {
				return nil, status.Error(codes.InvalidArgument, "expected no keys in request")
			}

			if len(policy.Fields) != 0 {
				return nil, status.Error(codes.InvalidArgument, "expected no fields in request")
			}

			return &api.TopicStatus{Id: topicID, State: api.TopicState_PENDING}, nil
		}

		state, err := s.client.SetTopicDeduplicationPolicy(ctx, topicID, api.Deduplication_DATAGRAM, api.Deduplication_OFFSET_LATEST, nil, false)
		require.NoError(err, "expected no error to set datagram with no keys or fields")
		require.Equal(api.TopicState_PENDING, state)
		require.Equal(1, s.mock.Calls[mock.SetTopicPolicyRPC])
	})

	s.Run("Keys", func() {
		defer s.mock.Reset()
		s.mock.OnSetTopicPolicy = func(ctx context.Context, tp *api.TopicPolicy) (*api.TopicStatus, error) {
			if tp.Id != topicID {
				return nil, status.Error(codes.InvalidArgument, "incorrect topic id")
			}

			if tp.DeduplicationPolicy == nil || tp.ShardingStrategy != api.ShardingStrategy_UNKNOWN {
				return nil, status.Error(codes.InvalidArgument, "missing deduplication policy or sharding strategy set")
			}

			policy := tp.DeduplicationPolicy
			if policy.Strategy != api.Deduplication_KEY_GROUPED || policy.Offset != api.Deduplication_OFFSET_LATEST {
				return nil, status.Error(codes.InvalidArgument, "deduplication policy not set correctly")
			}

			if len(policy.Keys) != 3 {
				return nil, status.Error(codes.InvalidArgument, "expected 3 keys in request")
			}

			if len(policy.Fields) != 0 {
				return nil, status.Error(codes.InvalidArgument, "expected no fields in request")
			}

			return &api.TopicStatus{Id: topicID, State: api.TopicState_PENDING}, nil
		}

		state, err := s.client.SetTopicDeduplicationPolicy(ctx, topicID, api.Deduplication_KEY_GROUPED, api.Deduplication_OFFSET_LATEST, names, false)
		require.NoError(err, "expected no error to set datagram with no keys or fields")
		require.Equal(api.TopicState_PENDING, state)
		require.Equal(1, s.mock.Calls[mock.SetTopicPolicyRPC])
	})

	s.Run("Fields", func() {
		defer s.mock.Reset()
		s.mock.OnSetTopicPolicy = func(ctx context.Context, tp *api.TopicPolicy) (*api.TopicStatus, error) {
			if tp.Id != topicID {
				return nil, status.Error(codes.InvalidArgument, "incorrect topic id")
			}

			if tp.DeduplicationPolicy == nil || tp.ShardingStrategy != api.ShardingStrategy_UNKNOWN {
				return nil, status.Error(codes.InvalidArgument, "missing deduplication policy or sharding strategy set")
			}

			policy := tp.DeduplicationPolicy
			if policy.Strategy != api.Deduplication_UNIQUE_FIELD || policy.Offset != api.Deduplication_OFFSET_LATEST {
				return nil, status.Error(codes.InvalidArgument, "deduplication policy not set correctly")
			}

			if len(policy.Keys) != 0 {
				return nil, status.Error(codes.InvalidArgument, "expected no keys in request")
			}

			if len(policy.Fields) != 3 {
				return nil, status.Error(codes.InvalidArgument, "expected 3 fields in request")
			}

			return &api.TopicStatus{Id: topicID, State: api.TopicState_PENDING}, nil
		}

		state, err := s.client.SetTopicDeduplicationPolicy(ctx, topicID, api.Deduplication_UNIQUE_FIELD, api.Deduplication_OFFSET_LATEST, names, false)
		require.NoError(err, "expected no error to set unique fields with 3 fields")
		require.Equal(api.TopicState_PENDING, state)
		require.Equal(1, s.mock.Calls[mock.SetTopicPolicyRPC])
	})

}

func (s *sdkTestSuite) TestSetTopicShardingStrategy() {
	require := s.Require()
	topicID := "01HCG64Y1SMFQBW7A42SRV207A"

	// Authenticate the client
	ctx := context.Background()
	require.NoError(s.Authenticate(ctx))

	s.Run("APIError", func() {
		defer s.mock.Reset()
		s.mock.UseError(mock.SetTopicPolicyRPC, codes.FailedPrecondition, "mock error")
		_, err := s.client.SetTopicShardingStrategy(ctx, topicID, api.ShardingStrategy_CONSISTENT_KEY_HASH)
		s.GRPCErrorIs(err, codes.FailedPrecondition, "mock error")
	})

	s.Run("HappyPath", func() {
		defer s.mock.Reset()
		s.mock.OnSetTopicPolicy = func(ctx context.Context, tp *api.TopicPolicy) (*api.TopicStatus, error) {
			if tp.Id != topicID {
				return nil, status.Error(codes.InvalidArgument, "incorrect topic id")
			}

			if tp.ShardingStrategy != api.ShardingStrategy_CONSISTENT_KEY_HASH {
				return nil, status.Error(codes.InvalidArgument, "expecting consistent key hash strategy")
			}

			if tp.DeduplicationPolicy != nil {
				return nil, status.Error(codes.InvalidArgument, "deduplication policy in request")
			}

			return &api.TopicStatus{Id: topicID, State: api.TopicState_PENDING}, nil
		}

		state, err := s.client.SetTopicShardingStrategy(ctx, topicID, api.ShardingStrategy_CONSISTENT_KEY_HASH)
		require.NoError(err, "expected no error to set unique fields with 3 fields")
		require.Equal(api.TopicState_PENDING, state)
		require.Equal(1, s.mock.Calls[mock.SetTopicPolicyRPC])
	})
}
