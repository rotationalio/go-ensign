package sdk_test

import (
	"context"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *sdkTestSuite) TestInfo() {
	require := s.Require()
	ctx := context.Background()

	// Authenticate the client for info tests
	err := s.Authenticate(ctx)
	require.NoError(err, "must be able to authenticate")

	// If the mock returns an error; then test info should return the error.
	s.mock.UseError(mock.InfoRPC, codes.FailedPrecondition, "could not process request")
	info, err := s.client.Info(ctx)
	s.GRPCErrorIs(err, codes.FailedPrecondition, "could not process request")
	require.Nil(info, "expected no info response returned")

	// Create a test handler for the ensign server
	s.mock.OnInfo = func(ctx context.Context, in *api.InfoRequest) (out *api.ProjectInfo, err error) {
		topics := []struct {
			ID       string
			readonly bool
			offset   uint64
		}{
			{"01GZ1ASDEPPFWD485HSQKDAS4K", false, 119},
			{"01GZ1B17QMNENAVY1AYN6C9DR5", true, 73},
			{"01GZ1B1CWHNP5QET60W4ZNCEKD", true, 2394},
			{"01GZ1B1J8SYRFJ53KKNG067SXT", false, 82122},
			{"01GZ1B1Q9NJ2CF9HAQRF720V60", false, 321},
		}

		out = &api.ProjectInfo{
			ProjectId: "01GZ1AQVTNF32YJWX6VP3Q7H4P",
		}

		includeTopics := make(map[string]bool)
		for _, tid := range in.Topics {
			topicID := ulid.ULID{}
			if err = topicID.UnmarshalBinary(tid); err != nil {
				return nil, status.Error(codes.InvalidArgument, "could not parse topic")
			}
			includeTopics[topicID.String()] = true
		}

		for _, topic := range topics {
			// Check filters
			if len(includeTopics) > 0 {
				if _, ok := includeTopics[topic.ID]; !ok {
					continue
				}
			}

			out.Topics++
			if topic.readonly {
				out.ReadonlyTopics++
			}
			out.Events += topic.offset
		}

		return out, nil
	}

	// The rest of the tests below use the test handler above
	// Test unfiltered requests
	info, err = s.client.Info(ctx)
	require.NoError(err)
	require.Equal(uint64(5), info.Topics)
	require.Equal(uint64(2), info.ReadonlyTopics)
	require.Equal(uint64(0x14c25), info.Events)

	// Test filtered requests with one topic
	info, err = s.client.Info(ctx, "01GZ1B17QMNENAVY1AYN6C9DR5")
	require.NoError(err)
	require.Equal(uint64(1), info.Topics)
	require.Equal(uint64(1), info.ReadonlyTopics)
	require.Equal(uint64(73), info.Events)

	// Test filtered request with multiple topics
	info, err = s.client.Info(ctx, "01GZ1B17QMNENAVY1AYN6C9DR5", "01GZ1B1Q9NJ2CF9HAQRF720V60", "01GZ1ASDEPPFWD485HSQKDAS4K")
	require.NoError(err)
	require.Equal(uint64(3), info.Topics)
	require.Equal(uint64(1), info.ReadonlyTopics)
	require.Equal(uint64(0x201), info.Events)

	// Test filtered request with unknown topic
	info, err = s.client.Info(ctx, "01GZ1BAP8757Q6R8N6ZCTFK92B")
	require.NoError(err)
	require.Equal(uint64(0), info.Topics)
	require.Equal(uint64(0), info.ReadonlyTopics)
	require.Equal(uint64(0), info.Events)

	// Invalid topic should not make a request to Ensign
	require.Equal(5, s.mock.Calls[mock.InfoRPC], "check prerequisite number of calls")
	_, err = s.client.Info(ctx, "01GZ1BAP8757Q6R8N6ZCTFK92B", "notaulid", "01GZ1B1Q9NJ2CF9HAQRF720V60")
	require.EqualError(err, `could not parse "notaulid" as a topic id`)
	require.Equal(5, s.mock.Calls[mock.InfoRPC], "an unexpected RPC call was made to Ensign")
}
