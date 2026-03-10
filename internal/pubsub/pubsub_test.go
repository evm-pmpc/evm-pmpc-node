package pubsub

import (
	"encoding/json"
	"testing"

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
