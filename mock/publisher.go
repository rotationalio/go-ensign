package mock

import (
	"errors"
	"io"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PublishHandler provides an OnPublish function that assists in the testing of publish
// streams by breaking down the initialization and messaging phase of the publisher
// stream. For example, this handler can be used to ensure that a specific number of
// events get published or to send acks or nacks to specific events.
type PublishHandler struct {
	OnInitialize func(in *api.OpenStream) (out *api.StreamReady, err error)
	OnEvent      func(in *api.EventWrapper) (out *api.PublisherReply, err error)
}

// By default new publish handlers ack all events and return the specified topic map.
func NewPublishHandler(topics map[string]ulid.ULID) *PublishHandler {
	return &PublishHandler{
		OnInitialize: func(in *api.OpenStream) (out *api.StreamReady, err error) {
			topicBytes := make(map[string][]byte)
			for name, id := range topics {
				topicBytes[name] = id.Bytes()
			}

			return &api.StreamReady{
				ClientId: in.ClientId,
				ServerId: "mock",
				Topics:   topicBytes,
			}, nil
		},
		OnEvent: func(in *api.EventWrapper) (out *api.PublisherReply, err error) {
			return &api.PublisherReply{
				Embed: &api.PublisherReply_Ack{
					Ack: &api.Ack{
						Id:        in.LocalId,
						Committed: timestamppb.Now(),
					},
				},
			}, nil
		},
	}
}

func (s *PublishHandler) OnPublish(stream api.Ensign_PublishServer) (err error) {
	// When stream is opened wait for the open stream message
	var msg *api.PublisherRequest
	if msg, err = stream.Recv(); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return status.Errorf(codes.Aborted, "stream canceled before initialization: %s", err)
	}

	// The first message should be an open stream message, if so use the OnInitialize
	// method, otherwise return an error from the mock.
	switch opn := msg.Embed.(type) {
	case *api.PublisherRequest_OpenStream:
		var reply *api.StreamReady
		if s.OnInitialize != nil {
			if reply, err = s.OnInitialize(opn.OpenStream); err != nil {
				return err
			}
		} else {
			reply = &api.StreamReady{ClientId: opn.OpenStream.ClientId, ServerId: "mock"}
		}

		if err = stream.Send(&api.PublisherReply{Embed: &api.PublisherReply_Ready{Ready: reply}}); err != nil {
			return status.Errorf(codes.Canceled, "could not send stream ready message: %s", err)
		}
	default:
		return status.Error(codes.FailedPrecondition, "expected an open stream message for initialization")
	}

	// Wait to receive events published to the server, then handle them.
	for {
		if msg, err = stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return status.Errorf(codes.Aborted, "publish stream aborted: %s", err)
		}

		switch req := msg.Embed.(type) {
		case *api.PublisherRequest_Event:
			var rep *api.PublisherReply
			if s.OnEvent != nil {
				if rep, err = s.OnEvent(req.Event); err != nil {
					return err
				}
			} else {
				rep = &api.PublisherReply{Embed: &api.PublisherReply_Nack{Nack: &api.Nack{Id: req.Event.LocalId, Code: api.Nack_UNPROCESSED}}}
			}

			if err = stream.Send(rep); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return status.Errorf(codes.Canceled, "could not send publish reply: %s", err)
			}
		default:
			return status.Error(codes.FailedPrecondition, "only events allowed after stream initialization")
		}
	}
}
