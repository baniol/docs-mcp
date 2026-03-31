package syncer

import (
	"context"
	"log/slog"
	"time"
)

// RepoClient is the minimal interface the syncer needs.
type RepoClient interface {
	Sync() (changed bool, err error)
	HeadHash() string
}

// OnUpdateFunc is called when the repo changes. Receives the new HEAD hash.
type OnUpdateFunc func(newHash string)

// RepoSyncer periodically pulls the repository and notifies on changes.
type RepoSyncer struct {
	client   RepoClient
	interval time.Duration
	onUpdate OnUpdateFunc
	lastHash string
}

func New(client RepoClient, interval time.Duration, onUpdate OnUpdateFunc) *RepoSyncer {
	return &RepoSyncer{
		client:   client,
		interval: interval,
		onUpdate: onUpdate,
		lastHash: client.HeadHash(),
	}
}

// Sync performs a single sync. Returns true if the repo changed.
func (s *RepoSyncer) Sync() bool {
	changed, err := s.client.Sync()
	if err != nil {
		slog.Warn("sync failed", "err", err)
		return false
	}
	if !changed {
		slog.Debug("repo already up to date")
		return false
	}
	newHash := s.client.HeadHash()
	slog.Info("repo updated", "old", s.lastHash[:min(8, len(s.lastHash))], "new", newHash[:min(8, len(newHash))])
	s.lastHash = newHash
	if s.onUpdate != nil {
		s.onUpdate(newHash)
	}
	return true
}

// Start runs periodic sync until ctx is cancelled.
func (s *RepoSyncer) Start(ctx context.Context) {
	slog.Info("starting periodic sync", "interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("syncer stopped")
			return
		case <-ticker.C:
			s.Sync()
		}
	}
}
