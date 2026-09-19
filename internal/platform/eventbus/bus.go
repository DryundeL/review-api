package eventbus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

type Handler func(ctx context.Context, eventName string, payload []byte) error

type Noop struct{}

func (Noop) Publish(context.Context, string, []byte) error { return nil }

type LoggingPublisher struct {
	Log *slog.Logger
}

func (p LoggingPublisher) Publish(ctx context.Context, topic string, payload []byte) error {
	log := p.Log
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "eventbus publish", "topic", topic, "payload_bytes", len(payload))
	return nil
}

type Dispatcher struct {
	mu       sync.RWMutex
	byTopic  map[string][]Handler
	fallback []Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{byTopic: make(map[string][]Handler)}
}

func (d *Dispatcher) Subscribe(eventName string, h Handler) {
	if h == nil || eventName == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.byTopic[eventName] = append(d.byTopic[eventName], h)
}

func (d *Dispatcher) Publish(ctx context.Context, topic string, payload []byte) error {
	d.mu.RLock()
	handlers := append([]Handler{}, d.byTopic[topic]...)
	fallback := append([]Handler{}, d.fallback...)
	d.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, topic, payload); err != nil {
			return fmt.Errorf("handler %q: %w", topic, err)
		}
	}
	for _, h := range fallback {
		if err := h(ctx, topic, payload); err != nil {
			return fmt.Errorf("fallback handler %q: %w", topic, err)
		}
	}
	return nil
}

type Fanout struct {
	Pubs []Publisher
}

func (f Fanout) Publish(ctx context.Context, topic string, payload []byte) error {
	for i, p := range f.Pubs {
		if p == nil {
			continue
		}
		if err := p.Publish(ctx, topic, payload); err != nil {
			return fmt.Errorf("fanout[%d]: %w", i, err)
		}
	}
	return nil
}
