package stream

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"google.golang.org/grpc"
)

// Publisher wraps an stream.PublishClient to maintain an open publish stream to an
// Ensign node.
type Publisher struct {
	client   PublishClient            // the client is used to call the Publish RPC to establish a stream
	copts    []grpc.CallOption        // call options to pass to the Publish RPC
	smu      sync.RWMutex             // guards updates to the stream
	stream   api.Ensign_PublishClient // the currently open stream, maintained open using reconnect
	stop     chan struct{}            // global stop signal to shutdown the publisher
	down     chan struct{}            // signal from receiver that the stream is down and needs to be reconnected
	wg       *sync.WaitGroup          // reusable wait group to wait until sender/receiver are down
	fmu      sync.RWMutex             // guards updates to the fatal error
	fatal    error                    // if the publisher has fatally errored and cannot reconnect
	pending  map[ulid.ULID]pubreply   // track acks/nacks from the publisher
	topics   map[string]ulid.ULID     // maps topic names to topic IDs from the server
	serverID string                   // the server this publisher is connected to
}

func NewPublisher(client PublishClient, opts ...grpc.CallOption) (*Publisher, error) {
	pub := &Publisher{
		client:  client,
		copts:   opts,
		stop:    make(chan struct{}, 1),
		down:    make(chan struct{}, 1),
		wg:      &sync.WaitGroup{},
		fatal:   nil,
		pending: make(map[ulid.ULID]pubreply),
	}

	if err := pub.OpenStream(); err != nil {
		return nil, err
	}

	pub.wg.Add(1)
	go pub.Start()
	return pub, nil
}

func (p *Publisher) Publish(topic string, event *api.Event) (_ <-chan *api.PublisherReply, err error) {
	// Create a local ID for acks and nacks
	localID := ulid.Make()

	// Attempt to determine the topicID from the string
	var topicID ulid.ULID
	if topicID, err = p.resolveTopic(topic); err != nil {
		return nil, err
	}

	// Create the event wrapper for the event
	env := &api.EventWrapper{
		TopicId: topicID.Bytes(),
		LocalId: localID.Bytes(),
	}

	if err = env.Wrap(event); err != nil {
		return nil, err
	}

	// Attempt to send the message to the publisher
	p.smu.RLock()
	if p.stream == nil {
		panic("cannot send event when stream is not open")
	}

	err = p.stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_Event{Event: env}})
	p.smu.RUnlock()

	// Handle any send errors by returning them to the user
	if err != nil {
		return nil, err
	}

	// Create ack and nack channels and return
	reply := make(chan *api.PublisherReply, 1)
	p.pending[localID] = pubreply(reply)

	return reply, nil
}

func (p *Publisher) Start() {
	if p.client == nil {
		panic("cannot start uninitialized publisher; use NewPublisher")
	}

	// Ensure the start go routine marks itself as done when it exits
	defer p.wg.Done()

	// Start a receiver channel; it is assumed that OpenStream has already been called.
	p.wg.Add(1)
	go p.receiver()

	// Maintain the publish stream connection
	for {
		select {
		case <-p.down:
			// If we're not able to reconnect in a timely fashion, set the fatal error.
			if err := p.Reconnect(); err != nil {
				p.setFatal(err)
				return
			}

			// Attempt to reopen the stream to the server
			if err := p.OpenStream(); err != nil {
				p.setFatal(err)
				return
			}

			// Restart the receiver, which should be stopped when we got the down msg.
			p.wg.Add(1)
			go p.receiver()

		case <-p.stop:
			return
		}
	}
}

func (p *Publisher) Close() error {
	// Send a stop signal so we do not reconnect on error
	p.stop <- struct{}{}

	// Attempt to send a close stream message
	p.smu.RLock()
	err := p.stream.CloseSend()
	p.smu.RUnlock()
	if err != nil {
		return err
	}

	// Wait until the publisher stops gracefully
	p.wg.Wait()
	return nil
}

// OpenStream returns a new publish bidirectional stream using the Ensign client. It
// uses the default timeout to establish the stream and returns an error if the stream
// could not be connected.
func (p *Publisher) OpenStream() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), ReconnectTimeout)
	defer cancel()

	p.smu.Lock()
	defer p.smu.Unlock()
	if p.stream, err = p.client.PublishStream(ctx, p.copts...); err != nil {
		return err
	}

	// Send an open stream request
	// TODO: how to allow user to specify client ID?
	// TODO: how to specify the allowed topics?
	open := &api.OpenStream{ClientId: ulid.Make().String()}
	if err = p.stream.Send(&api.PublisherRequest{Embed: &api.PublisherRequest_OpenStream{OpenStream: open}}); err != nil {
		return err
	}

	// Perform a first recv to make sure that we're allowed to access this node.
	var rep *api.PublisherReply
	if rep, err = p.stream.Recv(); err != nil {
		return err
	}

	var ready *api.StreamReady
	if ready = rep.GetReady(); ready == nil {
		return ErrStreamUninitialized
	}

	// Create topic map and server info
	p.serverID = ready.ServerId
	p.topics = make(map[string]ulid.ULID)
	for name, data := range ready.Topics {
		var topicID ulid.ULID
		if err = topicID.UnmarshalBinary(data); err == nil {
			p.topics[name] = topicID
		}
	}

	return nil
}

// Wait for the gRPC connection to reconnect to the Ensign node.
func (p *Publisher) Reconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), ReconnectTimeout)
	defer cancel()

	if !p.client.WaitForReconnect(ctx) {
		return ErrReconnect
	}
	return nil
}

// Err returns any fatal errors that are set on the publisher. If a non-nil error is
// returned then the publisher is not running and all events published will fail.
func (p *Publisher) Err() error {
	p.fmu.RLock()
	defer p.fmu.RUnlock()
	return p.fatal
}

// Topics returns the map of topic names to ULID
func (p *Publisher) Topics() map[string]ulid.ULID {
	p.smu.RLock()
	defer p.smu.RUnlock()
	return p.topics
}

// Fatal sets a fatal error on the publisher and is only used internally.
func (p *Publisher) setFatal(err error) {
	p.fmu.Lock()
	p.fatal = err
	p.fmu.Unlock()
}

func (p *Publisher) resolveTopic(topic string) (topicID ulid.ULID, err error) {
	// Attempt to parse the topicID from the string first
	if topicID, err = ulid.Parse(topic); err == nil {
		return topicID, nil
	}

	// Attempt to lookup the topicID from the topic map
	p.smu.RLock()
	defer p.smu.RUnlock()
	if topicID, ok := p.topics[topic]; ok {
		return topicID, nil
	}

	return topicID, ErrResolveTopic
}

func (p *Publisher) receiver() {
	defer p.wg.Done()
	for {
		// Use an rlock to make sure the currently active stream is accessed
		p.smu.RLock()
		if p.stream == nil {
			panic("publisher receiver running when stream is not open")
		}

		// Fetch the next server message or the error for handling
		in, err := p.stream.Recv()
		p.smu.RUnlock()

		if err != nil {
			// Assume clean shutdown when error is EOF, stop the go routine.
			if errors.Is(err, io.EOF) {
				return
			}

			// Otherwise log the error and send a reconnect signal before shutting down.
			// TODO: configure logging for go sdk
			// log.Debug().Err(err).Msg("could not recv message from publish stream, attempting reconnect")
			p.down <- struct{}{}
			return
		}

		// Otherwise handle the ack/nack from the server
		switch msg := in.Embed.(type) {
		case *api.PublisherReply_Ack:
			var localID ulid.ULID
			if err = localID.UnmarshalBinary(msg.Ack.Id); err != nil {
				// TODO: log instead of panic on error
				panic(err)
			}

			if pending, ok := p.pending[localID]; ok {
				pending <- in
				close(pending)
				delete(p.pending, localID)
			}

		case *api.PublisherReply_Nack:
			var localID ulid.ULID
			if err = localID.UnmarshalBinary(msg.Nack.Id); err != nil {
				// TODO: log instead of panic on error
				panic(err)
			}

			if pending, ok := p.pending[localID]; ok {
				pending <- in
				close(pending)
				delete(p.pending, localID)
			}

		case *api.PublisherReply_CloseStream:
			// TODO: handle close stream and logging for close stream
			// stats := msg.CloseStream
			// log.Debug().Uint64("n_events", stats.Events).Uint64("n_topics", stats.Topics).Uint64("n_consumers", stats.Consumers).Msg("publish stream closed")
		default:
			// TODO: configure logging for go sdk
			// log.Debug().Type("publisher_reply", in.Embed).Msg("unhandled publish stream message from server: ignoring")
		}
	}
}

type pubreply chan<- *api.PublisherReply
