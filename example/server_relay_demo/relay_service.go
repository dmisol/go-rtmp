package main

import (
	"fmt"
	"sync"
)

// TODO: Create this service per apps.
// In this example, this instance is singleton.
type RelayService struct {
	mu      sync.Mutex
	streams map[string]*Pubsub
	srv     *RelayServer
}

func NewRelayService(srv *RelayServer) *RelayService {
	return &RelayService{
		streams: make(map[string]*Pubsub),
		srv:     srv,
	}
}

func (s *RelayService) NewPubsub(key string) (*Pubsub, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.streams[key]; ok {
		return nil, fmt.Errorf("already published: %s", key)
	}

	pubsub := NewPubsub(s, key)

	s.streams[key] = pubsub

	return pubsub, nil
}

func (s *RelayService) GetPubsub(key string) (*Pubsub, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pubsub, ok := s.streams[key]
	if !ok {
		return nil, fmt.Errorf("get(), not published: %s", key)
	}

	return pubsub, nil
}

func (s *RelayService) RemovePubsub(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.streams[key]; !ok {
		return fmt.Errorf("remove(), not published: %s", key)
	}

	delete(s.streams, key)

	return nil
}
