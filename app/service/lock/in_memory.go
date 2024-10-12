package lock

import "sync"

type InMemoryLocker struct {
	syncedMap sync.Map
}

func NewInMemoryLocker() *InMemoryLocker {
	return &InMemoryLocker{}
}

// Lock locks the mutex for the given key.
func (m *InMemoryLocker) Lock(key string) {
	mu, _ := m.syncedMap.LoadOrStore(key, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
}

// Unlock unlocks the mutex for the given key.
func (m *InMemoryLocker) Unlock(key string) {
	mu, ok := m.syncedMap.Load(key)
	if ok {
		mu.(*sync.Mutex).Unlock()
	}
}
