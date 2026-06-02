package ears

import (
	"AgentTest/body/ears/earsAdapter"
	"sync"
	"time"
)

// Ears 实例：多例，每个实例对应一个具体的物理/逻辑通道
// Ears 实例：多例模式。
// 每个实例代表一个独立的音频流通道（例如：通道A监听麦克风，通道B监听远程摄像头）。
type Ears struct {
	mu          sync.RWMutex             // 读写锁：保证并发安全。Hear方法写，Snapshot方法读。
	name        string                   // 通道唯一标识：如 "Main_Mic" 或 "IoT_Door_Sensor"。
	adapter     earsAdapter.AudioAdapter // 策略接口：决定音频从哪来（适配器模式）。
	buffer      []float32                // 数据存储：存储原始脉冲编码调制 (PCM) 的浮点数据。
	sampleRate  int                      // 采样率：每秒钟采集的样本数（如 16000Hz 表示 16k）。
	maxDuration time.Duration            // 时间窗口：决定“耳朵”能记得多久以前的声音（滑动窗口）。
}

// NewEars 初始化并启动监听。
// 参数:
//
//	name: 通道名称。
//	adapter: 已实现的适配器（例如传入一个 MicAdapter 实例）。
//	sampleRate: 音频采样率。
//	duration: 缓冲区最大保留时长。
func NewEars(name string, adapter earsAdapter.AudioAdapter, sampleRate int, duration time.Duration) *Ears {
	e := &Ears{
		name:        name,
		adapter:     adapter,
		sampleRate:  sampleRate,
		maxDuration: duration,
		buffer:      make([]float32, 0), // 初始化为空切片
	}

	// 【关键设计】：异步启动适配器
	// 适配器通常会在内部开启一个死循环读取硬件流。
	// 我们将当前的 e.Hear 方法作为回调函数传给适配器。
	go e.adapter.Start(e.Hear)

	return e
}

// Hear 被动接收数据
func (e *Ears) Hear(samples []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buffer = append(e.buffer, samples...)

	// 自动修剪逻辑
	limit := int(float64(e.sampleRate) * e.maxDuration.Seconds())
	if len(e.buffer) > limit {
		e.buffer = e.buffer[len(e.buffer)-limit:]
	}
}

// GetAudioSnapshot 获取音频快照
// p 用于接收数据的预分配切片（可选）。如果传入的 p 长度足够，将直接填充 p 以实现零拷贝。
// 如果 p 为 nil 或长度为 0，则按原有逻辑申请新内存并返回。
func (e *Ears) GetAudioSnapshot(duration time.Duration, p []float32) []float32 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. 计算所需采样点数
	needed := int(float64(e.sampleRate) * duration.Seconds())
	available := len(e.buffer)

	if available == 0 {
		return []float32{}
	}
	if needed > available {
		needed = available
	}

	// 2. 确定目标切片
	var result []float32
	if p != nil && len(p) >= needed {
		// 【零拷贝路径】：使用传入的预分配空间
		result = p[:needed]
	} else {
		// 【标准路径】：动态申请内存
		result = make([]float32, needed)
	}

	// 3. 内存拷贝（这里是必须的，因为要从回环 buffer 提取数据）
	// 注意：虽然 copy 是内存移动，但我们避免了 MakeSlice 的堆分配压力
	copy(result, e.buffer[available-needed:])

	return result
}
