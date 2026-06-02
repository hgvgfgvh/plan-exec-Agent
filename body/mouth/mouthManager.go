package mouth

import (
	"AgentTest/body/mouth/talkAdapter"
	"sync"
)

type MouthManager struct {
	mu       sync.RWMutex
	Channels map[string]*Mouth
}

var mgmt *MouthManager

func GetManager() *MouthManager {
	return mgmt
}

func init() {
	// 初始化默认的嘴巴
	logSrc := &talkAdapter.LogAdapter{}
	// ttsSrc := &mouthAdapter.TTSAdapter{VoiceRate: 1}

	defaultMouth := NewMouth("MainMouth", logSrc)

	mgmt = &MouthManager{
		Channels: map[string]*Mouth{
			"Default": defaultMouth,
		},
	}
}
