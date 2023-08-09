package ensign_test

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
	"github.com/stretchr/testify/require"
)

// NewEvent returns a new random event for testing purposes.
func NewEvent() *ensign.Event {
	event := &ensign.Event{
		Metadata: ensign.Metadata{"length": "256"},
		Data:     make([]byte, 256),
		Mimetype: mimetype.ApplicationOctetStream,
		Type: &api.Type{
			Name:         "random",
			MajorVersion: 1,
		},
		Created: time.Now(),
	}

	rand.Read(event.Data)
	return event
}

func TestEventIDParsing(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected string
	}{
		{nil, ""},
		{[]byte{}, ""},
		{[]byte{0x42}, "42"},
		{[]byte{0x01, 0x83, 0x42, 0x5F, 0x66, 0x6F, 0x00, 0x6F, 0xEB, 0x6B}, "061m4qv6dw06ztvb"},
		{[]byte{0x01, 0x83, 0x42, 0x5F, 0x66, 0x6F, 0x00, 0x6F, 0xEB, 0x6B, 0x66, 0x6F, 0x00, 0x6F, 0xEB, 0x6B}, "0183425F666F006FEB6B666F006FEB6B"},
		{bytes.Repeat([]byte{0x42, 0xef}, 10), "42EF42EF42EF42EF42EF42EF42EF42EF42EF42EF"},
		{bytes.Repeat([]byte{0x42}, 10), "89144gj289144gj2"},
	}

	for i, tc := range testCases {
		evt := &api.EventWrapper{Id: tc.input}
		out := ensign.NewOutgoingEvent(evt, nil)

		require.Equal(t, tc.expected, out.ID(), "test case %d did not parse outgoing event correctly", i)

		inc := ensign.NewIncomingEvent(evt, nil)
		require.Equal(t, tc.expected, inc.ID(), "test case %d did not parse incoming event correctly", i)
	}
}

func TestTopicIDParsing(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected string
		topicID  ulid.ULID
		err      error
	}{
		{nil, "", ulid.ULID{}, ensign.ErrNoTopicID},
		{[]byte{}, "", ulid.ULID{}, ensign.ErrNoTopicID},
		{[]byte{0x41}, "41", ulid.ULID{}, ulid.ErrDataSize},
		{ulid.MustParse("01H2RT8KB5TZZT4NPNPCJD4A1B").Bytes(), "01H2RT8KB5TZZT4NPNPCJD4A1B", ulid.MustParse("01H2RT8KB5TZZT4NPNPCJD4A1B"), nil},
		{bytes.Repeat([]byte{0x42}, 16), "2289144GJ289144GJ289144GJ2", ulid.MustParse("2289144GJ289144GJ289144GJ2"), nil},
		{bytes.Repeat([]byte{0x42, 0xef}, 10), "42EF42EF42EF42EF42EF42EF42EF42EF42EF42EF", ulid.ULID{}, ulid.ErrDataSize},
	}

	for i, tc := range testCases {
		evt := &api.EventWrapper{TopicId: tc.input}
		out := ensign.NewOutgoingEvent(evt, nil)

		require.Equal(t, tc.expected, out.TopicID(), "test case %d did not parse outgoing event correctly", i)

		tid, err := out.TopicULID()
		if tc.err != nil {
			require.ErrorIs(t, err, tc.err)
		} else {
			require.Equal(t, tc.topicID, tid)
		}

		inc := ensign.NewIncomingEvent(evt, nil)
		require.Equal(t, tc.expected, inc.TopicID(), "test case %d did not parse incoming event correctly", i)

		tid, err = inc.TopicULID()
		if tc.err != nil {
			require.ErrorIs(t, err, tc.err)
		} else {
			require.Equal(t, tc.topicID, tid)
		}

	}
}
