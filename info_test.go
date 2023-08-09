package ensign_test

import (
	"context"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
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

	projectID := ulid.MustParse("01GZ1AQVTNF32YJWX6VP3Q7H4P")

	// Create a test handler for the ensign server
	s.mock.OnInfo = func(ctx context.Context, in *api.InfoRequest) (out *api.ProjectInfo, err error) {
		topics := []struct {
			ID         string
			readonly   bool
			offset     uint64
			duplicates uint64
			dataBytes  uint64
			types      []*api.EventTypeInfo
		}{
			{"01GZ1ASDEPPFWD485HSQKDAS4K", false, 119, 10, 1024, []*api.EventTypeInfo{
				{
					Type: &api.Type{
						Name:         "Document",
						MajorVersion: 1,
					},
					Mimetype:      mimetype.ApplicationJSON,
					Events:        119,
					Duplicates:    10,
					DataSizeBytes: 1024,
				},
			}},
			{"01GZ1B17QMNENAVY1AYN6C9DR5", true, 73, 20, 2048, []*api.EventTypeInfo{
				{
					Type: &api.Type{
						Name:         "Feed Item",
						MinorVersion: 1,
					},
					Mimetype:      mimetype.ApplicationProtobuf,
					Events:        70,
					Duplicates:    10,
					DataSizeBytes: 1024,
				},
				{
					Type: &api.Type{
						Name:         "Feed Item",
						MinorVersion: 2,
					},
					Mimetype:      mimetype.ApplicationProtobuf,
					Events:        3,
					Duplicates:    10,
					DataSizeBytes: 1024,
				},
			}},
			{"01GZ1B1CWHNP5QET60W4ZNCEKD", true, 2394, 30, 4096, []*api.EventTypeInfo{
				{
					Type: &api.Type{
						Name:         "Trades",
						MajorVersion: 2,
					},
					Mimetype:      mimetype.ApplicationJSON,
					Events:        2394,
					Duplicates:    30,
					DataSizeBytes: 4096,
				},
			}},
			{"01GZ1B1J8SYRFJ53KKNG067SXT", false, 82122, 40, 8192, []*api.EventTypeInfo{
				{
					Type: &api.Type{
						Name:         "Weather",
						MajorVersion: 3,
						PatchVersion: 1,
					},
					Mimetype:      mimetype.ApplicationMsgPack,
					Events:        82000,
					Duplicates:    40,
					DataSizeBytes: 8000,
				},
				{
					Type: &api.Type{
						Name:         "Forecast",
						MajorVersion: 2,
						PatchVersion: 1,
					},
					Mimetype:      mimetype.ApplicationJSON,
					Events:        122,
					DataSizeBytes: 192,
				},
			}},
			{"01GZ1B1Q9NJ2CF9HAQRF720V60", false, 321, 50, 16384, []*api.EventTypeInfo{
				{
					Type: &api.Type{
						Name:         "Message",
						PatchVersion: 1,
					},
					Mimetype:      mimetype.TextPlain,
					Events:        321,
					Duplicates:    50,
					DataSizeBytes: 16384,
				},
			}},
		}

		out = &api.ProjectInfo{
			ProjectId: projectID[:],
			Topics:    make([]*api.TopicInfo, 0),
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

			topicID := ulid.MustParse(topic.ID)
			info := &api.TopicInfo{
				TopicId:    topicID[:],
				ProjectId:  projectID[:],
				Events:     topic.offset,
				Duplicates: topic.duplicates,
				Types:      topic.types,
			}

			out.NumTopics++
			if topic.readonly {
				out.NumReadonlyTopics++
			}
			out.Events += topic.offset
			out.Duplicates += topic.duplicates
			out.DataSizeBytes += topic.dataBytes
			out.Topics = append(out.Topics, info)
		}

		return out, nil
	}

	// The rest of the tests below use the test handler above
	// Test unfiltered requests
	info, err = s.client.Info(ctx)
	require.NoError(err)
	require.Len(info.Topics, 5)
	require.Equal(uint64(2), info.NumReadonlyTopics)
	require.Equal(uint64(0x14c25), info.Events)
	require.Equal(uint64(150), info.Duplicates)
	require.Equal(uint64(31744), info.DataSizeBytes)
	require.Len(info.Topics, 5)
	require.Equal("Document", info.Topics[0].Types[0].Type.Name)

	// Test filtered requests with one topic
	info, err = s.client.Info(ctx, "01GZ1B17QMNENAVY1AYN6C9DR5")
	require.NoError(err)
	require.Len(info.Topics, 1)
	require.Equal(uint64(1), info.NumReadonlyTopics)
	require.Equal(uint64(73), info.Events)
	require.Equal(uint64(20), info.Duplicates)
	require.Equal(uint64(0x800), info.DataSizeBytes)
	require.Equal("Feed Item", info.Topics[0].Types[0].Type.Name)

	// Test filtered request with multiple topics
	info, err = s.client.Info(ctx, "01GZ1B17QMNENAVY1AYN6C9DR5", "01GZ1B1Q9NJ2CF9HAQRF720V60", "01GZ1ASDEPPFWD485HSQKDAS4K")
	require.NoError(err)
	require.Len(info.Topics, 3)
	require.Equal(uint64(1), info.NumReadonlyTopics)
	require.Equal(uint64(0x201), info.Events)
	require.Equal(uint64(80), info.Duplicates)
	require.Equal(uint64(19456), info.DataSizeBytes)
	require.Equal("Message", info.Topics[2].Types[0].Type.Name)

	// Test filtered request with unknown topic
	info, err = s.client.Info(ctx, "01GZ1BAP8757Q6R8N6ZCTFK92B")
	require.NoError(err)
	require.Empty(info.Topics)
	require.Equal(uint64(0), info.NumReadonlyTopics)
	require.Equal(uint64(0), info.Events)
	require.Equal(uint64(0), info.Duplicates)
	require.Equal(uint64(0), info.DataSizeBytes)

	// Invalid topic should not make a request to Ensign
	require.Equal(5, s.mock.Calls[mock.InfoRPC], "check prerequisite number of calls")
	_, err = s.client.Info(ctx, "01GZ1BAP8757Q6R8N6ZCTFK92B", "notaulid", "01GZ1B1Q9NJ2CF9HAQRF720V60")
	require.EqualError(err, `could not parse "notaulid" as a topic id`)
	require.Equal(5, s.mock.Calls[mock.InfoRPC], "an unexpected RPC call was made to Ensign")
}
