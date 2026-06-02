package skillpacks

import "sync"

var (
	mu     sync.RWMutex
	loaded []Pack
)

func snapshotPacks() []Pack {
	mu.RLock()
	defer mu.RUnlock()
	return append([]Pack(nil), loaded...)
}

func packByID(id string) (Pack, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, p := range loaded {
		if p.ID == id {
			return p, true
		}
	}
	return Pack{}, false
}
