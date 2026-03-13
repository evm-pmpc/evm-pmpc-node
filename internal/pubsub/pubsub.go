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
	MaxMessageSize = 1 << 20 
	MaxMessageRate = 10      
)

type Message struct {
	Type      string          `json:"type"`
	SenderID  string          `json:"sender_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

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

	return &PubSubService{
		ctx:       ctx,
		host:      h,
		ps:        ps,
		topics:    make(map[string]*libp2pPubsub.Topic),
		subs:      make(map[string]*libp2pPubsub.Subscription),
		handlers:  make(map[string][]MessageHandler),
		rateLimit: make(map[peer.ID]*rateLimiter),
	}, nil
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
		p.rateLimit[peerID] = &rateLimiter{count: 1, resetTime: now.Add(time.Second)}
		return false
	}

	rl.count++
	return rl.count > MaxMessageRate
}

func (p *PubSubService) readLoop(topicName string, sub *libp2pPubsub.Subscription) {
	for {
		msg, err := sub.Next(p.ctx)
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			zap.L().Warn("error reading from pubsub topic", zap.String("topic", topicName), zap.Error(err))
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

		p.mu.RLock()
		handlers := p.handlers[topicName]
		p.mu.RUnlock()

		for _, handler := range handlers {
			handler(&parsedMsg)
		}
	}
}

func (p *PubSubService) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, sub := range p.subs {
		sub.Cancel()
		zap.L().Debug("cancelled subscription", zap.String("topic", name))
	}

	for name, topic := range p.topics {
		topic.Close()
		zap.L().Debug("closed topic", zap.String("topic", name))
	}
}
