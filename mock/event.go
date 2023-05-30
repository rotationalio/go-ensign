package mock

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
	region "github.com/rotationalio/go-ensign/region/v1beta1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var defaultFactory *EventFactory = &EventFactory{
	Topic:  ulid.MustParse("01H1PPYFQM8ZNXXPH6JJF2BEDN"),
	Region: region.Region_STG_LKE_US_EAST_1A,
}

// NewEventWrapper returns an event wrapper with random data inside. It is a quick method to
// create an event for the default "testing.123" topic.
func NewEventWrapper() *api.EventWrapper {
	return defaultFactory.Make()
}

// NewEvent returns an event with random data
func NewEvent() *api.Event {
	return defaultFactory.Event()
}

// EventFactory creates random events with standard defaults.
type EventFactory struct {
	sync.Mutex
	Topic  ulid.ULID
	Region region.Region
	epoch  uint64
	offset uint64
}

func (f *EventFactory) Make() *api.EventWrapper {
	f.Lock()
	defer f.Unlock()
	f.offset++
	committed := time.Now()
	created := committed.Add(time.Duration(-1*rand.Int63n(10000)) * time.Millisecond)

	env := &api.EventWrapper{
		Id:      ulid.Make().Bytes(),
		TopicId: f.Topic.Bytes(),
		Offset:  f.offset,
		Epoch:   f.epoch,
		Region:  f.Region,
		Publisher: &api.Publisher{
			PublisherId: "mock",
			Ipaddr:      "127.0.0.1",
			ClientId:    "test",
			UserAgent:   "mock",
		},
		Key:   nil,
		Shard: 0,
		Event: nil,
		Encryption: &api.Encryption{
			EncryptionAlgorithm: api.Encryption_PLAINTEXT,
			SealingAlgorithm:    api.Encryption_PLAINTEXT,
			SignatureAlgorithm:  api.Encryption_PLAINTEXT,
		},
		Compression: &api.Compression{
			Algorithm: api.Compression_NONE,
		},
		Committed: timestamppb.New(committed),
	}

	e := f.Event()
	e.Created = timestamppb.New(created)
	env.Wrap(e)

	return env
}

func (f *EventFactory) Event() *api.Event {
	e := &api.Event{
		Data:     make([]byte, 256),
		Mimetype: mimetype.ApplicationOctetStream,
		Metadata: map[string]string{
			"length": "256",
		},
		Type: &api.Type{
			Name:         "random",
			MajorVersion: 1,
		},
		Created: timestamppb.Now(),
	}

	rand.Read(e.Data)
	return e
}
