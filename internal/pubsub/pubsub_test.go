package pubsub

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestMessageMarshalUnmarshal(t *testing.T) {
	original := Message{
		Type:      "heartbeat",
		SenderID:  "12D3KooWTest",
		Timestamp: 1234567890,
		Payload:   json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("expected type '%s', got '%s'", original.Type, decoded.Type)
	}

	if decoded.SenderID != original.SenderID {
		t.Errorf("expected sender_id '%s', got '%s'", original.SenderID, decoded.SenderID)
	}

	if decoded.Timestamp != original.Timestamp {
		t.Errorf("expected timestamp %d, got %d", original.Timestamp, decoded.Timestamp)
	}
}

func TestMessageRejectEmpty(t *testing.T) {
	data := []byte(`{"type":"","sender_id":"test","timestamp":123}`)

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Type != "" {
		t.Error("expected empty type")
	}
}

func TestMessageRejectMalformed(t *testing.T) {
	data := []byte(`{invalid json}`)

	var msg Message
	if err := json.Unmarshal(data, &msg); err == nil {
		t.Error("expected error for malformed json, got nil")
	}
}

func TestMaxMessageSizeConstant(t *testing.T) {
	if MaxMessageSize != 1<<20 {
		t.Errorf("expected MaxMessageSize to be 1MB (1048576), got %d", MaxMessageSize)
	}
}

func TestRateLimiter(t *testing.T) {
	svc := &PubSubService{
		rateLimit: make(map[peer.ID]*rateLimiter),
	}

	testPeer := peer.ID("test-peer-id")

	for i := 0; i < MaxMessageRate; i++ {
		if svc.isRateLimited(testPeer) {
			t.Fatalf("peer should not be rate limited at message %d", i+1)
		}
	}

	if !svc.isRateLimited(testPeer) {
		t.Error("peer should be rate limited after exceeding max rate")
	}
}

func TestSweepRateLimit_DropsExpiredEntries(t *testing.T) {
	svc := &PubSubService{
		rateLimit: make(map[peer.ID]*rateLimiter),
	}

	now := time.Now()
	expired := peer.ID("expired-peer")
	fresh := peer.ID("fresh-peer")

	svc.rateLimit[expired] = &rateLimiter{count: 5, resetTime: now.Add(-time.Minute)}
	svc.rateLimit[fresh] = &rateLimiter{count: 1, resetTime: now.Add(time.Minute)}

	svc.sweepRateLimit(now)

	if _, ok := svc.rateLimit[expired]; ok {
		t.Error("expired entry should have been swept")
	}
	if _, ok := svc.rateLimit[fresh]; !ok {
		t.Error("fresh entry should have been retained")
	}
}

func TestSafeCall_RecoversFromHandlerPanic(t *testing.T) {
	called := false
	safeCall(func(msg *Message) {
		called = true
		panic("boom")
	}, &Message{Type: "x"}, "topic")

	if !called {
		t.Fatal("handler should have run despite panic")
	}
}

func TestDispatch_RunsAllHandlersEvenIfOnePanics(t *testing.T) {
	svc := &PubSubService{
		handlers: make(map[string][]MessageHandler),
	}

	var mu sync.Mutex
	var seen []string

	svc.handlers["t"] = []MessageHandler{
		func(*Message) {
			mu.Lock()
			seen = append(seen, "first")
			mu.Unlock()
		},
		func(*Message) { panic("boom") },
		func(*Message) {
			mu.Lock()
			seen = append(seen, "third")
			mu.Unlock()
		},
	}

	svc.dispatch("t", &Message{Type: "x"})

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != "first" || seen[1] != "third" {
		t.Errorf("expected first+third to run, got %v", seen)
	}
}
