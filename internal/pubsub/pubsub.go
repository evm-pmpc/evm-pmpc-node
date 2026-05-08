package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	libp2pPubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// MaxMessageSize is the hard ceiling on a single inbound or outbound
	// pubsub message, in bytes.
	MaxMessageSize = 1 << 20 // 1 MB

	// MaxMessageRate is the per-peer message budget per rate-limit window.
	MaxMessageRate = 10

	// rateLimitWindow is the length of one rate-limit window. Combined with
	// MaxMessageRate, this is a fixed-window per-peer limiter.
	rateLimitWindow = time.Second

	// rateLimitSweepInterval bounds how often we GC stale per-peer entries
	// from the rate-limit map. Without this the map grows unboundedly with
	// the number of unique peers seen across the node's lifetime.
	rateLimitSweepInterval = 5 * time.Minute

	// readLoopBackoff is the ctx-aware sleep applied after a non-fatal
	// sub.Next error so the loop doesn't tight-spin on a flaky subscription.
	readLoopBackoff = 100 * time.Millisecond
)

type Message struct {
	Type      string          `json:"type"`
	SenderID  string          `json:"sender_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// MessageHandler is invoked once per matching pubsub message. Handlers are
// called synchronously inside the per-topic read loop, so a slow handler
// throttles its topic — keep handler bodies cheap, or hand work off to a
// goroutine internally.
type MessageHandler func(msg *Message)

type PubSubService struct {
	ctx       context.Context
	host      host.Host
	ps        *libp2pPubsub.PubSub
	topics    map[string]*libp2pPubsub.Topic
	subs      map[string]*libp2pPubsub.Subscription
	handlers  map[string][]MessageHandler
	rateLimit map[peer.ID]*rateLimiter
	mu        sync.RWMutex

	closeOnce sync.Once
	stopCh    chan struct{}
}

type rateLimiter struct {
	count     int
	resetTime time.Time
}

func NewPubSubService(ctx context.Context, h host.Host) (*PubSubService, error) {
	ps, err := libp2pPubsub.NewGossipSub(ctx, h,
		libp2pPubsub.WithMaxMessageSize(MaxMessageSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GossipSub: %w", err)
	}

	svc := &PubSubService{
		ctx:       ctx,
		host:      h,
		ps:        ps,
		topics:    make(map[string]*libp2pPubsub.Topic),
		subs:      make(map[string]*libp2pPubsub.Subscription),
		handlers:  make(map[string][]MessageHandler),
		rateLimit: make(map[peer.ID]*rateLimiter),
		stopCh:    make(chan struct{}),
	}
	go svc.runRateLimitSweeper()
	return svc, nil
}

func (p *PubSubService) JoinTopic(topicName string) (*libp2pPubsub.Topic, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if topic, exists := p.topics[topicName]; exists {
		return topic, nil
	}

	topic, err := p.ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}
	p.topics[topicName] = topic

	sub, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		delete(p.topics, topicName)
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}
	p.subs[topicName] = sub

	go p.readLoop(topicName, sub)

	zap.L().Info("joined pubsub topic", zap.String("topic", topicName))
	return topic, nil
}

func (p *PubSubService) Subscribe(topicName string, handler MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topicName] = append(p.handlers[topicName], handler)
}

func (p *PubSubService) Publish(topicName string, msgType string, payload interface{}) error {
	p.mu.RLock()
	topic, exists := p.topics[topicName]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not joined to topic %s", topicName)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := Message{
		Type:      msgType,
		SenderID:  p.host.ID().String(),
		Timestamp: time.Now().Unix(),
		Payload:   payloadBytes,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if len(msgBytes) > MaxMessageSize {
		return fmt.Errorf("message exceeds max size (%d > %d bytes)", len(msgBytes), MaxMessageSize)
	}

	return topic.Publish(p.ctx, msgBytes)
}

func (p *PubSubService) ListPeers(topicName string) []peer.ID {
	p.mu.RLock()
	topic, exists := p.topics[topicName]
	p.mu.RUnlock()

	if !exists {
		return nil
	}

	return topic.ListPeers()
}

func (p *PubSubService) isRateLimited(peerID peer.ID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	rl, exists := p.rateLimit[peerID]
	now := time.Now()

	if !exists || now.After(rl.resetTime) {
		p.rateLimit[peerID] = &rateLimiter{count: 1, resetTime: now.Add(rateLimitWindow)}
		return false
	}

	rl.count++
	return rl.count > MaxMessageRate
}

// runRateLimitSweeper periodically drops rate-limit entries whose window has
// long since expired. This keeps the map's footprint proportional to the
// recently-active peer set rather than the cumulative one.
func (p *PubSubService) runRateLimitSweeper() {
	ticker := time.NewTicker(rateLimitSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			p.sweepRateLimit(now)
		}
	}
}

func (p *PubSubService) sweepRateLimit(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, rl := range p.rateLimit {
		if now.After(rl.resetTime) {
			delete(p.rateLimit, id)
		}
	}
}

func (p *PubSubService) readLoop(topicName string, sub *libp2pPubsub.Subscription) {
	for {
		msg, err := sub.Next(p.ctx)
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			zap.L().Warn("error reading from pubsub topic", zap.String("topic", topicName), zap.Error(err))
			if !sleepCtx(p.ctx, readLoopBackoff) {
				return
			}
			continue
		}

		if msg.ReceivedFrom == p.host.ID() {
			continue
		}

		if p.isRateLimited(msg.ReceivedFrom) {
			zap.L().Warn("rate limited peer", zap.String("peer", msg.ReceivedFrom.String()))
			continue
		}

		if len(msg.Data) > MaxMessageSize {
			zap.L().Warn("dropped oversized message", zap.String("peer", msg.ReceivedFrom.String()), zap.Int("size", len(msg.Data)))
			continue
		}

		var parsedMsg Message
		if err := json.Unmarshal(msg.Data, &parsedMsg); err != nil {
			zap.L().Warn("received malformed pubsub message", zap.String("topic", topicName), zap.Error(err))
			continue
		}

		if parsedMsg.Type == "" {
			zap.L().Warn("rejected message with empty type", zap.String("peer", msg.ReceivedFrom.String()))
			continue
		}

		p.dispatch(topicName, &parsedMsg)
	}
}

// dispatch fans the message out to every handler for a topic. Each handler
// is wrapped in a recover() so a panicking handler can't take down the
// per-topic read loop or the whole process.
func (p *PubSubService) dispatch(topicName string, msg *Message) {
	p.mu.RLock()
	handlers := make([]MessageHandler, len(p.handlers[topicName]))
	copy(handlers, p.handlers[topicName])
	p.mu.RUnlock()

	for _, handler := range handlers {
		safeCall(handler, msg, topicName)
	}
}

func safeCall(handler MessageHandler, msg *Message, topicName string) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("pubsub handler panicked",
				zap.String("topic", topicName),
				zap.Any("panic", r),
			)
		}
	}()
	handler(msg)
}

func (p *PubSubService) Close() {
	p.closeOnce.Do(func() {
		close(p.stopCh)

		// Snapshot under the lock, then release the lock before calling
		// into libp2p.Topic/Subscription Close — those can block and may
		// re-enter this service.
		p.mu.Lock()
		subs := p.subs
		topics := p.topics
		p.subs = nil
		p.topics = nil
		p.handlers = nil
		p.rateLimit = nil
		p.mu.Unlock()

		for name, sub := range subs {
			sub.Cancel()
			zap.L().Debug("cancelled subscription", zap.String("topic", name))
		}
		for name, topic := range topics {
			if err := topic.Close(); err != nil {
				zap.L().Debug("failed to close topic", zap.String("topic", name), zap.Error(err))
			}
		}
	})
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
