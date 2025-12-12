package consensus

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestLeadershipRotator_Start(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("creates rotator with valid interval", func(t *testing.T) {
		node := &Node{}
		interval := 1 * time.Second

		rotator := NewLeadershipRotator(node, interval, logger)

		if rotator == nil {
			t.Fatal("expected rotator to be created")
		}

		if rotator.interval != interval {
			t.Errorf("expected interval %v, got %v", interval, rotator.interval)
		}

		if rotator.node != node {
			t.Error("expected node to be set")
		}
	})

	t.Run("fails with zero interval", func(t *testing.T) {
		node := &Node{}
		interval := 0 * time.Second

		rotator := NewLeadershipRotator(node, interval, logger)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := rotator.Start(ctx)
		if err == nil {
			t.Error("expected error for zero interval")
		}
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		node := &Node{}
		interval := 10 * time.Second

		rotator := NewLeadershipRotator(node, interval, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := rotator.Start(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("stops when Stop() is called", func(t *testing.T) {
		node := &Node{}
		interval := 10 * time.Second

		rotator := NewLeadershipRotator(node, interval, logger)

		ctx := context.Background()

		done := make(chan error, 1)
		go func() {
			done <- rotator.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)
		rotator.Stop()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("rotator did not stop in time")
		}
	})
}

func TestLeadershipRotator_transferLeadership(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("skips transfer when not leader", func(t *testing.T) {
		node := &Node{
			raft: nil,
		}

		rotator := NewLeadershipRotator(node, 1*time.Second, logger)

		err := rotator.transferLeadership()
		if err != nil {
			t.Errorf("expected no error when not leader, got %v", err)
		}
	})
}
