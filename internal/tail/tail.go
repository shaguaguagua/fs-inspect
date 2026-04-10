// Package tail subscribes to the ESL event stream on every node in the
// cluster inventory and merges the streams onto a single channel.
//
// This is the backing library for `fs-inspect tail`. It intentionally
// only speaks the lowest-common-denominator shape — one Event per ESL
// message, with raw header access — and leaves formatting, filtering,
// and TUI rendering to callers.
package tail

import (
	"context"
	"fmt"
	"sync"
	"time"

	eventsocket "github.com/fiorix/go-eventsocket/eventsocket"

	"github.com/shaguaguagua/fs-inspect/internal/config"
)

// Event is one ESL event tagged with the node it came from. A non-nil
// Err means the upstream subscription for that node died; the node's
// goroutine has already exited by the time you see it.
//
// Name is the Event-Name header, duplicated out of Header for
// convenience. Header values should be read via the Get helper because
// fiorix stores them as interface{} and may return []string for
// repeated keys.
type Event struct {
	Node   config.Node
	Time   time.Time
	Name   string
	Header eventsocket.EventHeader
	Body   string
	Err    error
}

// Get returns a header value by key, or the empty string. fiorix
// normalises header names so "Unique-ID" must be looked up as
// "Unique-Id" and so on.
func (e Event) Get(key string) string {
	val, ok := e.Header[key]
	if !ok || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	if ss, ok := val.([]string); ok && len(ss) > 0 {
		return ss[0]
	}
	return ""
}

// Subscribe dials every node in cfg, subscribes to the given space-
// separated ESL event names (e.g. "CHANNEL_CREATE CHANNEL_HANGUP_COMPLETE"),
// and merges the event streams onto the returned channel.
//
// Call stop() to tear down every subscription. stop() cancels the shared
// context, waits for every producer goroutine to exit, then closes the
// channel. Callers must still range over the channel until it closes;
// stop() does not drain pending events on its own.
func Subscribe(cfg *config.Config, events string) (<-chan Event, func()) {
	out := make(chan Event, 128)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(n config.Node) {
			defer wg.Done()
			runNode(ctx, n, events, out)
		}(node)
	}

	stop := func() {
		cancel()
		wg.Wait()
		close(out)
	}
	return out, stop
}

// runNode holds one ESL connection open, forwards matching events to
// out, and exits cleanly on context cancellation. A watcher goroutine
// closes the underlying connection on ctx.Done() to unblock any
// outstanding ReadEvent call, which is otherwise un-cancellable.
func runNode(ctx context.Context, node config.Node, events string, out chan<- Event) {
	conn, err := eventsocket.Dial(node.Addr, node.Password)
	if err != nil {
		emit(ctx, out, Event{Node: node, Time: time.Now(), Err: fmt.Errorf("dial: %w", err)})
		return
	}
	defer conn.Close()

	if _, err := conn.Send("event plain " + events); err != nil {
		emit(ctx, out, Event{Node: node, Time: time.Now(), Err: fmt.Errorf("subscribe: %w", err)})
		return
	}

	// Watcher: unblock ReadEvent when ctx is cancelled by closing the
	// underlying connection out from under it.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		ev, err := conn.ReadEvent()
		if err != nil {
			if ctx.Err() != nil {
				return // clean shutdown via watcher
			}
			emit(ctx, out, Event{Node: node, Time: time.Now(), Err: fmt.Errorf("read: %w", err)})
			return
		}
		emit(ctx, out, Event{
			Node:   node,
			Time:   time.Now(),
			Name:   ev.Get("Event-Name"),
			Header: ev.Header,
			Body:   ev.Body,
		})
	}
}

// emit sends ev on out unless ctx is already cancelled. This keeps
// producer goroutines from deadlocking when the consumer has stopped
// reading but hasn't yet invoked stop().
func emit(ctx context.Context, out chan<- Event, ev Event) {
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}
