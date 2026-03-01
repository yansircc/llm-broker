package events

import (
	"testing"
	"time"
)

func TestPublish_AddsToRing(t *testing.T) {
	bus := NewBus(10)
	bus.Publish(Event{Type: EventBan, Message: "test ban"})

	recent := bus.Recent(0)
	if len(recent) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recent))
	}
	if recent[0].Type != EventBan {
		t.Errorf("expected ban event, got %s", recent[0].Type)
	}
}

func TestRecent_RespectLimit(t *testing.T) {
	bus := NewBus(100)
	for i := range 10 {
		bus.Publish(Event{Type: EventRequest, Message: "req", Timestamp: time.Now().Add(time.Duration(i) * time.Second)})
	}

	recent := bus.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 events, got %d", len(recent))
	}
}

func TestRing_WrapsCorrectly(t *testing.T) {
	bus := NewBus(5)
	for i := range 8 {
		bus.Publish(Event{Type: EventRequest, Message: "msg", Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond)})
	}

	recent := bus.Recent(0)
	if len(recent) != 5 {
		t.Fatalf("expected 5 events (ring size), got %d", len(recent))
	}
	// Events should be in order
	for i := 1; i < len(recent); i++ {
		if recent[i].Timestamp.Before(recent[i-1].Timestamp) {
			t.Error("events should be in chronological order")
		}
	}
}

func TestSubscribe_ReceivesEvents(t *testing.T) {
	bus := NewBus(10)
	_, ch, _ := bus.Subscribe()

	bus.Publish(Event{Type: EventRecover, Message: "recovered"})

	select {
	case e := <-ch:
		if e.Type != EventRecover {
			t.Errorf("expected recover event, got %s", e.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("subscriber did not receive event")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus(10)
	id, ch, _ := bus.Subscribe()
	bus.Unsubscribe(id)

	bus.Publish(Event{Type: EventBan, Message: "ban"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("should not receive events after unsubscribe")
		}
		// Channel closed — expected
	case <-time.After(100 * time.Millisecond):
		// Also acceptable — channel closed
	}
}
