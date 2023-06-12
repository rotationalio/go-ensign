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

func TestTypeSemver(t *testing.T) {
	testCases := []struct {
		name     string
		vers     string
		expected string
		semver   string
	}{
		{"GameStarted", "1.4.2", "GameStarted v1.4.2", "1.4.2"},
		{"GameStarted", "0.0.4", "GameStarted v0.0.4", "0.0.4"},
		{"GameStarted", "1.2.3", "GameStarted v1.2.3", "1.2.3"},
		{"GameStarted", "10.20.30", "GameStarted v10.20.30", "10.20.30"},
		{"GameStarted", "1.1.2-prerelease+meta", "GameStarted v1.1.2", "1.1.2"},
		{"GameStarted", "1.1.2+meta-valid", "GameStarted v1.1.2", "1.1.2"},
		{"GameStarted", "1.1.2-alpha", "GameStarted v1.1.2", "1.1.2"},
		{"GameStarted", "1.1.2-beta", "GameStarted v1.1.2", "1.1.2"},
		{"GameStarted", "1.1.2-alpha.1", "GameStarted v1.1.2", "1.1.2"},
		{"GameStarted", "1.1.2-rc.1+build.123", "GameStarted v1.1.2", "1.1.2"},
		{"Foo", "999999999.999999999.999999999", "Foo v999999999.999999999.999999999", "999999999.999999999.999999999"},
	}

	for i, tc := range testCases {
		eventType := &api.Type{Name: tc.name}

		err := eventType.ParseSemver(tc.vers)
		require.NoError(t, err, "could not parse semver for test case %d", i)

		require.Equal(t, tc.expected, eventType.Version(), "mismatched version in test case %d", i)
		require.Equal(t, tc.semver, eventType.Semver(), "mismatched semver in test case %d", i)
	}
}

func TestTypeBadSemver(t *testing.T) {
	testCases := []string{
		"1.0", "a.b.c", "", "not a version", "2", "1.2.3.4",
		"99999999999999999999999.999999999999999999.99999999999999999",
		"1.999999999999999999.99999999999999999",
		"1.1.99999999999999999",
	}

	for _, tc := range testCases {
		eventType := &api.Type{Name: "Bad version"}
		err := eventType.ParseSemver(tc)
		require.Error(t, err, "expected semver parsing error for %q", tc)
	}
}
