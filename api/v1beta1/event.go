package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/proto"
)

func (w *EventWrapper) Wrap(e *Event) (err error) {
	if w.Event, err = proto.Marshal(e); err != nil {
		return err
	}
	return nil
}

func (w *EventWrapper) Unwrap() (e *Event, err error) {
	if len(w.Event) == 0 {
		return nil, errors.New("event wrapper contains no event")
	}

	e = &Event{}
	if err = proto.Unmarshal(w.Event, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (w *EventWrapper) ParseTopicID() (topicID ulid.ULID, err error) {
	err = topicID.UnmarshalBinary(w.TopicId)
	return topicID, err
}

func (t *Type) Version() string {
	return fmt.Sprintf("%s v%d.%d.%d", t.Name, t.MajorVersion, t.MinorVersion, t.PatchVersion)
}

// Equals treats the name as case-insensitive.
func (t *Type) Equals(o *Type) bool {
	tname := strings.TrimSpace(strings.ToLower(t.Name))
	oname := strings.TrimSpace(strings.ToLower(o.Name))

	return (tname == oname &&
		t.MajorVersion == o.MajorVersion &&
		t.MinorVersion == o.MinorVersion &&
		t.PatchVersion == o.PatchVersion)
}
