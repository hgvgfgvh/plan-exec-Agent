package earsAdapter

import (
	"fmt"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// MicAdapter 本地麦克风真实适配器
type MicAdapter struct {
	sourceName  string
	sampleRate  int
	isRecording bool
	ctx         *malgo.AllocatedContext
	device      *malgo.Device
	stopChan    chan struct{}
}

// NewMicAdapter 构造函数
func NewMicAdapter(sampleRate int) *MicAdapter {
	return &MicAdapter{
		sourceName: "Physical_Microphone",
		sampleRate: sampleRate,
		stopChan:   make(chan struct{}),
	}
}

// Start 开启物理麦克风监听
func (a *MicAdapter) Start(callback func([]float32)) error {
	if a.isRecording {
		return fmt.Errorf("mic is already recording")
	}

	// 1. 初始化物理上下文
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("audio context init failed: %v", err)
	}
	a.ctx = ctx

	// 2. 硬件参数配置
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatF32 // 关键：要求声卡直接输出浮点流
	deviceConfig.Capture.Channels = 1             // 单声道
	deviceConfig.SampleRate = uint32(a.sampleRate)

	// 3. 定义内部回调函数
	onData := func(pOutputSample, pInputSamples []byte, frameCount uint32) {
		if callback == nil || len(pInputSamples) == 0 {
			return
		}

		// --- 核心转换逻辑 (不改变外部框架) ---
		// malgo 返回的是 []byte，但内容是 F32 原始二进制。
		// 使用 unsafe.Slice 将 []byte 指针重新解释为 []float32，无需复制内存。
		// float32 占 4 字节，所以长度为 byte 长度 / 4
		samples := unsafe.Slice((*float32)(unsafe.Pointer(&pInputSamples[0])), len(pInputSamples)/4)

		// 将处理好的浮点切片推向框架的 Ears.Hear 方法
		callback(samples)
	}

	// 4. 绑定回调并初始化设备
	a.device, err = malgo.InitDevice(a.ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onData,
	})
	if err != nil {
		a.ctx.Uninit()
		a.ctx.Free()
		return fmt.Errorf("device init failed: %v", err)
	}

	// 5. 启动硬件
	if err := a.device.Start(); err != nil {
		return fmt.Errorf("device start failed: %v", err)
	}

	a.isRecording = true

	// 资源释放监听
	go func() {
		<-a.stopChan
		a.device.Stop()
		a.device.Uninit()
		a.ctx.Uninit()
		a.ctx.Free()
		a.isRecording = false
	}()

	return nil
}

// Stop 停止录音并释放资源
func (a *MicAdapter) Stop() error {
	if a.isRecording {
		close(a.stopChan)
	}
	return nil
}

// GetSourceType 返回来源描述
func (a *MicAdapter) GetSourceType() string {
	return "HARDWARE_MIC_REALTIME"
}
