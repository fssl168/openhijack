package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type PromptEntry struct {
	Hash string
	Text string
}

type SystemPromptStore struct {
	mu        sync.RWMutex
	prompts   map[string]string
	overrides map[string]string
}

func NewSystemPromptStore() *SystemPromptStore {
	return &SystemPromptStore{
		prompts:   make(map[string]string),
		overrides: make(map[string]string),
	}
}

func (s *SystemPromptStore) ComputeHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func (s *SystemPromptStore) Capture(hash, text string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.prompts[hash]; exists {
		return false
	}
	s.prompts[hash] = text
	return true
}

func (s *SystemPromptStore) SetOverride(hash, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides[hash] = text
}

func (s *SystemPromptStore) GetOverride(hash string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	text, ok := s.overrides[hash]
	return text, ok
}

func (s *SystemPromptStore) CaptureAndGetOverrides(entries []PromptEntry) ([]string, map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var added []string
	for _, e := range entries {
		if _, exists := s.prompts[e.Hash]; !exists {
			s.prompts[e.Hash] = e.Text
			added = append(added, e.Hash)
		}
	}

	active := make(map[string]string)
	for k, v := range s.overrides {
		if _, exists := s.prompts[k]; exists {
			active[k] = v
		}
	}
	return added, active
}
