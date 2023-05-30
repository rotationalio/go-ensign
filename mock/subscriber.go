package mock

import (
	"encoding/base64"
	"errors"
	"io"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SubscribeHandler provides an OnSubscribe function that assists in the testing of
// subscribe streams by breaking down the initialization and messaging phases of the
// subscription stream. For example, this handler can be used to send 10 events before
// quitting or to send events at a routine interval.
type SubscribeHandler struct {
	OnInitialize func(in *api.Subscription) (out *api.StreamReady, err error)
	OnAck        func(in *api.Ack) (err error)
	OnNack       func(in *api.Nack) (err error)
	Send         chan<- *api.EventWrapper
	events       <-chan *api.EventWrapper
}

func NewSubscribeHandler() *SubscribeHandler {
	events := make(chan *api.EventWrapper, 64)
	return &SubscribeHandler{
		Send:   events,
		events: events,
	}
}

// UseTopicMap sets OnInitialize to use the topics in the topic map, returning an error
// if the subscription contains topics that are not in the topics map.
func (s *SubscribeHandler) UseTopicMap(topics map[string]ulid.ULID) {
	s.OnInitialize = func(in *api.Subscription) (out *api.StreamReady, err error) {
		// Filter topics from subscription
		filter := make(map[string]struct{})
		for _, name := range in.Topics {
			if _, ok := topics[name]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "unknown topic %q", name)
			}
			filter[name] = struct{}{}
		}

		out = &api.StreamReady{
			ClientId: in.ClientId,
			ServerId: "mock",
			Topics:   make(map[string][]byte),
		}

		for name, id := range topics {
			if len(filter) > 0 {
				if _, keep := filter[name]; !keep {
					continue
				}
			}

			out.Topics[name] = id.Bytes()
		}
		return out, nil
	}
}

// This method should be added to the mock as the OnSubscribe handler.
func (s *SubscribeHandler) OnSubscribe(stream api.Ensign_SubscribeServer) (err error) {
	// When the stream is opened wait for the subscription message
	var msg *api.SubscribeRequest
	if msg, err = stream.Recv(); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return status.Error(codes.Aborted, "stream canceled before initialization")
	}

	// The first message should be a subscription message; if so use the OnInitialize
	// method otherwise return an error from the mock.
	switch sub := msg.Embed.(type) {
	case *api.SubscribeRequest_Subscription:
		var reply *api.StreamReady
		if s.OnInitialize != nil {
			if reply, err = s.OnInitialize(sub.Subscription); err != nil {
				return err
			}
		} else {
			reply = &api.StreamReady{ClientId: sub.Subscription.ClientId, ServerId: "mock"}
		}

		if err = stream.Send(&api.SubscribeReply{Embed: &api.SubscribeReply_Ready{Ready: reply}}); err != nil {
			return status.Error(codes.Canceled, "could not send stream ready message")
		}
	default:
		return status.Error(codes.FailedPrecondition, "expected a subscription to initialize the stream")
	}

	// Once initialized launch a go routine to send messages that come in from the send channel
	go func() {
		stats := &api.CloseStream{Consumers: 1}
		topics := make(map[string]struct{})

		for event := range s.events {
			if err := stream.Send(&api.SubscribeReply{Embed: &api.SubscribeReply_Event{Event: event}}); err != nil {
				return
			}

			stats.Events++
			topics[base64.RawStdEncoding.EncodeToString(event.TopicId)] = struct{}{}
		}

		// Once the events channel has been closed send close stream message
		stats.Topics = uint64(len(topics))
		stream.Send(&api.SubscribeReply{Embed: &api.SubscribeReply_CloseStream{CloseStream: stats}})
	}()

	// Receive acks/nacks etc and handle them with the callbacks
	for {
		if msg, err = stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return status.Error(codes.Aborted, "subscribe stream aborted")
		}

		switch req := msg.Embed.(type) {
		case *api.SubscribeRequest_Ack:
			if s.OnAck != nil {
				if err = s.OnAck(req.Ack); err != nil {
					return err
				}
			}
		case *api.SubscribeRequest_Nack:
			if s.OnNack != nil {
				if err = s.OnNack(req.Nack); err != nil {
					return err
				}
			}
		default:
			return status.Error(codes.FailedPrecondition, "only acks/nacks allowed after stream initialization")
		}
	}
}

// Shutdown the stream by sending a close stream message
func (s *SubscribeHandler) Shutdown() {
	close(s.Send)
}
