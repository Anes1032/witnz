package verify

import (
	"fmt"
	"log/slog"

	"github.com/witnz/witnz/internal/alert"
	"github.com/witnz/witnz/internal/storage"
)

type FollowerVerifier struct {
	storage      *storage.Storage
	alertManager *alert.Manager
	shutdownFunc func() error
	logger       *slog.Logger
	autoShutdown bool
}

func NewFollowerVerifier(
	store *storage.Storage,
	alertMgr *alert.Manager,
	shutdownFunc func() error,
	autoShutdown bool,
	logger *slog.Logger,
) *FollowerVerifier {
	if logger == nil {
		logger = slog.Default()
	}

	return &FollowerVerifier{
		storage:      store,
		alertManager: alertMgr,
		shutdownFunc: shutdownFunc,
		autoShutdown: autoShutdown,
		logger:       logger,
	}
}

func (v *FollowerVerifier) VerifyHashEntry(tableName string, seqNum uint64, consensusHash string) error {
	localEntry, err := v.storage.GetHashEntry(tableName, seqNum)
	if err != nil {
		return fmt.Errorf("failed to get local hash entry: %w", err)
	}

	if localEntry == nil {
		v.logger.Warn("Local hash entry not found",
			"table", tableName,
			"sequence", seqNum,
		)
		return nil
	}

	if localEntry.Hash != consensusHash {
		v.logger.Error("Inconsistency detected between local hash and consensus hash",
			"table", tableName,
			"sequence", seqNum,
			"local_hash", localEntry.Hash,
			"consensus_hash", consensusHash,
		)

		if v.alertManager != nil {
			v.alertManager.SendSystemAlert(
				"Follower Inconsistency Detected",
				fmt.Sprintf(
					"Follower detected inconsistency with Leader at %s:%d. Leader is authority. Self-terminating.\n"+
						"Local hash: %s\nConsensus hash: %s",
					tableName, seqNum, localEntry.Hash, consensusHash,
				),
				"danger",
			)
		}

		if v.autoShutdown && v.shutdownFunc != nil {
			v.logger.Warn("Auto-shutdown enabled, initiating shutdown")

			if err := v.storage.SetMetadata("follower_terminated_due_to_inconsistency", "true"); err != nil {
				v.logger.Error("Failed to set termination flag", "error", err)
			}

			if err := v.shutdownFunc(); err != nil {
				return fmt.Errorf("shutdown failed: %w", err)
			}
		}

		return fmt.Errorf("inconsistency detected: local hash %s != consensus hash %s", localEntry.Hash, consensusHash)
	}

	v.logger.Debug("Hash entry verified successfully",
		"table", tableName,
		"sequence", seqNum,
	)

	return nil
}

func (v *FollowerVerifier) CheckTerminationFlag() (bool, error) {
	flag, err := v.storage.GetMetadata("follower_terminated_due_to_inconsistency")
	if err != nil {
		return false, fmt.Errorf("failed to check termination flag: %w", err)
	}

	return flag == "true", nil
}
