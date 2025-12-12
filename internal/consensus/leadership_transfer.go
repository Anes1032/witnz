package consensus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/raft"
)

type LeadershipRotator struct {
	node     *Node
	interval time.Duration
	stopCh   chan struct{}
	logger   *slog.Logger
}

func NewLeadershipRotator(node *Node, interval time.Duration, logger *slog.Logger) *LeadershipRotator {
	if logger == nil {
		logger = slog.Default()
	}

	return &LeadershipRotator{
		node:     node,
		interval: interval,
		stopCh:   make(chan struct{}),
		logger:   logger,
	}
}

func (r *LeadershipRotator) Start(ctx context.Context) error {
	if r.interval <= 0 {
		return fmt.Errorf("invalid interval: %v", r.interval)
	}

	r.logger.Info("Leadership rotator started", "interval", r.interval)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.transferLeadership(); err != nil {
				r.logger.Error("Leadership transfer failed", "error", err)
			}
		case <-r.stopCh:
			r.logger.Info("Leadership rotator stopped")
			return nil
		case <-ctx.Done():
			r.logger.Info("Leadership rotator stopped due to context cancellation")
			return ctx.Err()
		}
	}
}

func (r *LeadershipRotator) transferLeadership() error {
	if r.node.raft == nil {
		r.logger.Debug("Raft not initialized, skipping leadership transfer")
		return nil
	}

	if r.node.raft.State() != raft.Leader {
		r.logger.Debug("Not the leader, skipping leadership transfer")
		return nil
	}

	currentLeader := r.node.config.NodeID

	r.logger.Info("Initiating leadership transfer", "current_leader", currentLeader)

	future := r.node.raft.LeadershipTransfer()
	if err := future.Error(); err != nil {
		return fmt.Errorf("leadership transfer failed: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	newLeaderAddr := r.node.raft.Leader()
	r.logger.Info("Leadership transferred successfully",
		"old_leader", currentLeader,
		"new_leader", newLeaderAddr,
	)

	return nil
}

func (r *LeadershipRotator) Stop() {
	close(r.stopCh)
}
