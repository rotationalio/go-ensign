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

// Subscriber wraps a stream.SubscribeClient to maintain an open subscribe stream to an
// Ensign node. When the subscriber is started it kicks off a go routine that watches
// for when the stream goes down and attempts to reconnect it gracefully. This go
// routine also spins off go routines for receiving messages from the stream. The
// received events are passed to a channel that must be consumed by the caller; if the
// channel fills up, the event will be dropped and a nack sent back to the server.
//
// Sending acks/nacks back to the server happens synchronously in the user thread, an
// error is returned if the message cannot be sent.
type Subscriber struct {
	client       SubscribeClient            // the client is used to call the Subscribe RPC to establish a stream
	copts        []grpc.CallOption          // call options passed to the Subscribe RPC
	subscription *api.Subscription          // the subscription info to initialize the stream (e.g. consumer groups, topics, etc.)
	smu          sync.RWMutex               // guards updates to the stream
	stream       api.Ensign_SubscribeClient // the currently open stream, maintained open using reconnect
	events       chan<- *api.EventWrapper   // the channel received events are sent on
	stop         chan struct{}              // global stop signal to shutdown the subscriber
	down         chan struct{}              // signal from the receiver that the stream is down and needs to be reconnected
	wg           *sync.WaitGroup            // reusable wait group to wait until the start and receive go routines are stopped
	fmu          sync.RWMutex               // guards updates to the fatal error
	fatal        error                      // if the subscriber has fatally errored and cannot reconnect
	topics       map[string]ulid.ULID       // maps topic names to topic IDs from the server
	serverID     string                     // the server this subscriber is connected to
}

// Create a new low-level subscribe stream manager that maintains an open subscribe
// stream and allows user to send acks/nack back to the Ensign node. This function opens
// a subscribe stream and returns an error if the user is not authenticated or the
// stream cannot be opened. If the stream is opened successfully, the start go routine
// is kicked off, which ensures the stream stays open even if the remote node
// temporarily goes down. The start go routine also kicks off the receive routine to
// get events from the server, which are sent down the returned event channel.
//
// NOTE: it is the caller's responsibility to consume the returned event channel; if the
// buffer gets filled up events may be dropped and nacked back to the server.
func NewSubscriber(client SubscribeClient, topics []string, opts ...grpc.CallOption) (_ <-chan *api.EventWrapper, _ *Subscriber, err error) {
	sub := &Subscriber{
		client: client,
		copts:  opts,
		stop:   make(chan struct{}, 1),
		down:   make(chan struct{}, 1),
		wg:     &sync.WaitGroup{},
		fatal:  nil,
	}

	// Create the subscription to reconnect the stream with.
	// TODO: map topic names to IDs for a better subscription experience
	// TODO: handle consumer groups, queries, and other subscribe options.
	sub.subscription = &api.Subscription{
		ClientId: ulid.Make().String(),
		Topics:   topics,
	}

	if err = sub.openStream(); err != nil {
		return nil, nil, err
	}

	// Create the channel to send received events on
	events := make(chan *api.EventWrapper, BufferSize)
	sub.events = events

	// Start go routines
	sub.wg.Add(1)
	go sub.start()

	return events, sub, nil
}

// Ack sends an acknowledgement to the server via the subscribe stream. This method
// blocks until a stream is available to send on and synchronously sends the ack.
func (c *Subscriber) Ack(ack *api.Ack) error {
	req := &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Ack{
			Ack: ack,
		},
	}

	c.smu.RLock()
	defer c.smu.RUnlock()
	if c.stream == nil {
		panic("cannot send ack when stream is not open")
	}

	return c.stream.Send(req)
}

// Nack sends an event handling error to the server via the subscribe stream. This
// method blocks until a stream is available to send on and synchronously sends the nack.
func (c *Subscriber) Nack(nack *api.Nack) error {
	req := &api.SubscribeRequest{
		Embed: &api.SubscribeRequest_Nack{
			Nack: nack,
		},
	}

	c.smu.RLock()
	defer c.smu.RUnlock()
	if c.stream == nil {
		panic("cannot send nack when stream is not open")
	}

	return c.stream.Send(req)
}

// Close the subscriber gracefully, once closed, the subscriber cannot be restarted.
func (c *Subscriber) Close() error {
	// Send a stop signal so that we do not reconnect on error
	c.stop <- struct{}{}

	// Attempt to send a close stream message
	c.smu.RLock()
	err := c.stream.CloseSend()
	c.smu.RUnlock()

	if err != nil {
		return err
	}

	// Wait until subscriber stops gracefully
	c.wg.Wait()

	// Close the events channel to signal to any go routines that the subscriber is done.
	close(c.events)
	return nil
}

// Err returns any fatal errors that are set on the subscriber. If a non-nil error is
// returned then the subscriber is not running so no events will be received and no
// messages can be sent to the server.
func (c *Subscriber) Err() error {
	c.fmu.RLock()
	defer c.fmu.RUnlock()
	return c.fatal
}

// Topics returns the map of topic names to ULID that is sent by the server when the
// stream is opened and correctly initialized.
func (c *Subscriber) Topics() map[string]ulid.ULID {
	c.smu.RLock()
	defer c.smu.RUnlock()
	return c.topics
}

// The start go routine manages the stream and receive go routine. If the receive go
// routine cannot recv a message from the server, this routine waits until the
// connection is reestablished then reopens the stream and restarts the receive routine.
func (c *Subscriber) start() {
	// Ensure the start go routine marks itself as done when it exits
	defer c.wg.Done()

	// Start a receiver channel; it is assumed that openStream has already been called.
	c.wg.Add(1)
	go c.receiver()

	// Maintain the subscribe stream connection
	for {
		select {
		case <-c.down:
			// If we're not able to reconnect in a timely fashion, set the fatal error.
			if err := c.reconnect(); err != nil {
				c.setFatal(err)
				return
			}

			// Attempt to reopen the stream to the server
			if err := c.openStream(); err != nil {
				c.setFatal(err)
				return
			}

			// Restart the receiver, which should have been stopped when we got the down signal.
			c.wg.Add(1)
			go c.receiver()

		case <-c.stop:
			return
		}
	}
}

// openStream returns a new subscribe bidirectional stream using the Ensign client. It
// uses the default timeout to establish the stream and returns an error if the stream
// could not be connected. Once connected, it sends a subscription message to the server
// and waits until it receives the stream ready response from the server. If it fails
// to open the stream or the subscription cannot be established an error is returned.
func (c *Subscriber) openStream() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), ReconnectTimeout)
	defer cancel()

	c.smu.Lock()
	defer c.smu.Unlock()
	if c.stream, err = c.client.SubscribeStream(ctx, c.copts...); err != nil {
		return err
	}

	// Send the subscription request to the server
	req := &api.SubscribeRequest{Embed: &api.SubscribeRequest_Subscription{Subscription: c.subscription}}
	if err = c.stream.Send(req); err != nil {
		return err
	}

	var rep *api.SubscribeReply
	if rep, err = c.stream.Recv(); err != nil {
		return err
	}

	var ready *api.StreamReady
	if ready = rep.GetReady(); ready == nil {
		return ErrStreamUninitialized
	}

	// Create topic map and server info
	c.serverID = ready.ServerId
	c.topics = make(map[string]ulid.ULID)
	for name, data := range ready.Topics {
		var topicID ulid.ULID
		if err = topicID.UnmarshalBinary(data); err == nil {
			c.topics[name] = topicID
		}
	}

	return nil
}

// Wait for the gRPC connection to reconnect to the Ensign node.
func (c *Subscriber) reconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), ReconnectTimeout)
	defer cancel()

	if !c.client.WaitForReconnect(ctx) {
		return ErrReconnect
	}
	return nil
}

// The receiver go routine listens for subscribe events and sends them to the events
// channel. It is this routine's responsibility to detect if the stream is down on an
// error by recv. If so, the routine quits and sends a signal to the start routine to
// reconnect. Note that if the events buffer is full, this routine will block forever.
func (c *Subscriber) receiver() {
	defer c.wg.Done()
	for {
		// use an rlock to make sure the currently active stream is accessed
		c.smu.RLock()
		if c.stream == nil {
			panic("subscriber receiver running when stream is not open")
		}

		// Fetch the next event from the server or the error for handling
		in, err := c.stream.Recv()
		c.smu.RUnlock()

		if err != nil {
			// Assume a clean shutdown when error is EOF, stop go routine
			if errors.Is(err, io.EOF) {
				return
			}

			// Otherwise log the error and send a reconnect signal before shutting down.
			// TODO: configure logging for go sdk
			// log.Debug().Err(err).Msg("could not recv message from subscribe stream, attempting reconnect")
			c.down <- struct{}{}
			return
		}

		// Handle the message from the server
		switch msg := in.Embed.(type) {
		case *api.SubscribeReply_Event:
			c.events <- msg.Event
		case *api.SubscribeReply_CloseStream:
			// TODO: handle close stream and logging for close stream
			// stats := msg.CloseStream
			// log.Debug().Uint64("n_events", stats.Events).Uint64("n_topics", stats.Topics).Uint64("n_consumers", stats.Consumers).Msg("subscribe stream closed")
		default:
			// TODO: configure logging for go sdk
			// log.Debug().Type("subscriber_reply", in.Embed).Msg("unhandled subscribe stream message from server: ignoring")
		}
	}
}

// Sets a fatal error on the subscriber and is only used internally.
func (c *Subscriber) setFatal(err error) {
	c.fmu.Lock()
	c.fatal = err
	c.fmu.Unlock()
}
