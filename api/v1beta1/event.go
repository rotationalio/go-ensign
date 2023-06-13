package api

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/proto"
)

// Wrap an event to create a complete protocol buffer to send to the Ensign server.
func (w *EventWrapper) Wrap(e *Event) (err error) {
	if w.Event, err = proto.Marshal(e); err != nil {
		return err
	}
	return nil
}

// Unwrap an event from the event wrapper for user consumption.
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

// Parse the TopicID as a ULID.
func (w *EventWrapper) ParseTopicID() (topicID ulid.ULID, err error) {
	err = topicID.UnmarshalBinary(w.TopicId)
	return topicID, err
}

// Returns the type name and semantic version as a whole string.
func (t *Type) Version() string {
	return fmt.Sprintf("%s v%d.%d.%d", t.Name, t.MajorVersion, t.MinorVersion, t.PatchVersion)
}

// Returns just the semantic version of the type.
func (t *Type) Semver() string {
	return fmt.Sprintf("%d.%d.%d", t.MajorVersion, t.MinorVersion, t.PatchVersion)
}

var (
	ErrSemverParse = errors.New("could not parse version string as a semantic version 2.0.0")
	semverPattern  = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// Parses the semver 2.0.0 string and loads the information into the type.
// See https://semver.org/ and https://regex101.com/r/Ly7O1x/3/ for more on parsing.
// NOTE: any extensions of the version such as build and release are omitted, only the
// major, minor, and patch versions are added to the type.
func (t *Type) ParseSemver(version string) (err error) {
	if !semverPattern.MatchString(version) {
		return ErrSemverParse
	}

	matches := semverPattern.FindStringSubmatch(version)
	if t.MajorVersion, err = parseUint32(matches[1]); err != nil {
		return ErrSemverParse
	}

	if t.MinorVersion, err = parseUint32(matches[2]); err != nil {
		return ErrSemverParse
	}

	if t.PatchVersion, err = parseUint32(matches[3]); err != nil {
		return ErrSemverParse
	}

	return nil
}

func parseUint32(s string) (uint32, error) {
	i, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(i), nil
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
