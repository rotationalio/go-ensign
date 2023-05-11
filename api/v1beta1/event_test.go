package api_test

import (
	"crypto/rand"
	"testing"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestEventWrapper(t *testing.T) {
	// Create an event wrapper and random event data
	wrap := &api.EventWrapper{
		Id:      ulid.Make().Bytes(),
		TopicId: ulid.Make().Bytes(),
		Offset:  421,
		Epoch:   23,
	}

	evt := &api.Event{
		Data: make([]byte, 128),
	}
	rand.Read(evt.Data)

	err := wrap.Wrap(evt)
	require.NoError(t, err, "should be able to wrap an event in an event wrapper")

	cmp, err := wrap.Unwrap()
	require.NoError(t, err, "should be able to unwrap an event in an event wrapper")
	require.NotNil(t, cmp, "the unwrapped event should not be nil")
	require.True(t, proto.Equal(evt, cmp), "the unwrapped event should match the original")
	require.NotSame(t, evt, cmp, "a pointer to the same event should not be returned")

	topicID, err := wrap.ParseTopicID()
	require.NoError(t, err, "could not parse topic id")
	require.Equal(t, wrap.TopicId, topicID.Bytes(), "parsed topicID does not match original")

	wrap.Event = nil
	empty, err := wrap.Unwrap()
	require.EqualError(t, err, "event wrapper contains no event")
	require.Empty(t, empty, "no data event should be zero-valued")
}

func TestType(t *testing.T) {
	car := &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8}
	require.Equal(t, "car v1.4.8", car.Version())

	// Equality checking
	testCases := []struct {
		alpha  *api.Type
		bravo  *api.Type
		assert require.BoolAssertionFunc
	}{
		{
			alpha:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			assert: require.True,
		},
		{
			alpha:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "Car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			assert: require.True,
		},
		{
			alpha:  &api.Type{Name: "CAR", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			assert: require.True,
		},
		{
			alpha:  &api.Type{Name: " car ", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car ", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			assert: require.True,
		},
		{
			alpha:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car", MajorVersion: 2, MinorVersion: 4, PatchVersion: 8},
			assert: require.False,
		},
		{
			alpha:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 9, PatchVersion: 8},
			assert: require.False,
		},
		{
			alpha:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 8},
			bravo:  &api.Type{Name: "car", MajorVersion: 1, MinorVersion: 4, PatchVersion: 0},
			assert: require.False,
		},
	}

	for i, tc := range testCases {
		tc.assert(t, tc.alpha.Equals(tc.bravo), "test case %d failed", i)
	}
}
