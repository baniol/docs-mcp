package syncer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockClient struct {
	syncCalled int32
	changed    bool
	err        error
	hash       string
}

func (m *mockClient) Sync() (bool, error) {
	atomic.AddInt32(&m.syncCalled, 1)
	return m.changed, m.err
}

func (m *mockClient) HeadHash() string { return m.hash }

func TestSyncer_SyncNoChange(t *testing.T) {
	client := &mockClient{hash: "abc123", changed: false}
	var called int32
	s := New(client, time.Hour, func(_ string) { atomic.AddInt32(&called, 1) })

	changed := s.Sync()
	if changed {
		t.Error("expected no change")
	}
	if called != 0 {
		t.Error("expected callback not to be called")
	}
}

func TestSyncer_SyncWithChange(t *testing.T) {
	client := &mockClient{hash: "newHash1234", changed: true}
	var called int32
	s := New(client, time.Hour, func(_ string) { atomic.AddInt32(&called, 1) })

	changed := s.Sync()
	if !changed {
		t.Error("expected change to be detected")
	}
	if called != 1 {
		t.Errorf("expected callback called once, got %d", called)
	}
}

func TestSyncer_ErrorResilience(t *testing.T) {
	client := &mockClient{hash: "abc", err: context.DeadlineExceeded}
	var called int32
	s := New(client, time.Hour, func(_ string) { atomic.AddInt32(&called, 1) })

	// Should not panic
	changed := s.Sync()
	if changed {
		t.Error("expected no change on error")
	}
	if called != 0 {
		t.Error("callback should not be called on error")
	}
}

func TestSyncer_ContextCancellation(t *testing.T) {
	client := &mockClient{hash: "abc"}
	s := New(client, 10*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("syncer did not stop after context cancellation")
	}
}
