package api

import "sync"

type DeliveryStore struct {
	mu         sync.Mutex
	deliveries map[string]struct{}
}

func NewDeliveryStore() *DeliveryStore {
	return &DeliveryStore{
		deliveries: make(map[string]struct{}),
	}
}

func (s *DeliveryStore) Seen(deliveryID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.deliveries[deliveryID]; exists {
		return true
	}

	s.deliveries[deliveryID] = struct{}{}
	return false
}

func (s *DeliveryStore) Forget(deliveryID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.deliveries, deliveryID)
}
