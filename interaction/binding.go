package interaction

import (
	"sync"
	"time"
)

const bindingTTL = 2 * time.Hour

type bindingStore struct {
	mu   sync.RWMutex
	byID map[string]*bindingEntry
}

type bindingEntry struct {
	ReplyBinding
	expires time.Time
}

func newBindingStore() *bindingStore {
	return &bindingStore{byID: make(map[string]*bindingEntry)}
}

func (s *bindingStore) Put(b ReplyBinding) {
	if s == nil || b.TurnID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[b.TurnID] = &bindingEntry{
		ReplyBinding: b,
		expires:      time.Now().Add(bindingTTL),
	}
}

func (s *bindingStore) Get(turnID string) (ReplyBinding, bool) {
	if s == nil || turnID == "" {
		return ReplyBinding{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byID[turnID]
	if !ok {
		return ReplyBinding{}, false
	}
	if time.Now().After(e.expires) {
		delete(s.byID, turnID)
		return ReplyBinding{}, false
	}
	return e.ReplyBinding, true
}

func (s *bindingStore) Delete(turnID string) {
	if s == nil || turnID == "" {
		return
	}
	s.mu.Lock()
	delete(s.byID, turnID)
	s.mu.Unlock()
}

func (s *bindingStore) gc() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.byID {
		if e == nil || now.After(e.expires) {
			delete(s.byID, id)
		}
	}
}
