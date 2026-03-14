package store

import (
	"log"
	"time"
)

func (s *Store) StartRetentionCleanup(retention time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once on startup
	s.runCleanup(retention)

	for {
		select {
		case <-ticker.C:
			s.runCleanup(retention)
		case <-stop:
			return
		}
	}
}

func (s *Store) runCleanup(retention time.Duration) {
	before := time.Now().Add(-retention)
	if err := s.DeleteOlderThan(before); err != nil {
		log.Printf("retention cleanup error: %v", err)
	}
}
