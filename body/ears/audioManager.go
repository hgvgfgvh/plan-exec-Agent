package ears

import (
	"AgentTest/body/ears/earsAdapter"
	"sync"
	"time"
)

type AudioManager struct {
	mu       sync.RWMutex
	Channels map[string]*Ears
}

var (
	mgmt *AudioManager
)

func GetManager() *AudioManager {
	return mgmt
}

func init() {
	// 注册真实麦克风
	micSrc := earsAdapter.NewMicAdapter(16000)
	micEars := NewEars("MainMic", micSrc, 16000, 5*time.Second)

	mgmt = &AudioManager{
		Channels: map[string]*Ears{
			"MicAdapter": micEars,
		},
	}
}
