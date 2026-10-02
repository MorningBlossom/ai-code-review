package api

import (
	"sync"
	"time"
)

const (
	deliveryStoreMaxEntries = 10_000
	deliveryStoreTTL        = 24 * time.Hour
)

type deliveryRecord struct {
	seenAt time.Time
}

type DeliveryStore struct {
	mu         sync.Mutex
	deliveries map[string]deliveryRecord
}

func NewDeliveryStore() *DeliveryStore {
	return &DeliveryStore{
		deliveries: make(map[string]deliveryRecord),
	}
}

func (s *DeliveryStore) Seen(deliveryID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Remove expired delivery IDs.
	for id, record := range s.deliveries {
		if now.Sub(record.seenAt) > deliveryStoreTTL {
			delete(s.deliveries, id)
		}
	}

	if _, exists := s.deliveries[deliveryID]; exists {
		return true
	}

	// Prevent unbounded memory growth.
	if len(s.deliveries) >= deliveryStoreMaxEntries {
		s.removeOldest()
	}

	s.deliveries[deliveryID] = deliveryRecord{
		seenAt: now,
	}

	return false
}

func (s *DeliveryStore) Forget(deliveryID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.deliveries, deliveryID)
}

func (s *DeliveryStore) removeOldest() {
	var oldestID string
	var oldestTime time.Time

	for id, record := range s.deliveries {
		if oldestID == "" || record.seenAt.Before(oldestTime) {
			oldestID = id
			oldestTime = record.seenAt
		}
	}

	if oldestID != "" {
		delete(s.deliveries, oldestID)
	}
}
