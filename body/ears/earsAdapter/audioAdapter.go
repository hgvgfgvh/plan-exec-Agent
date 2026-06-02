package earsAdapter

// AudioAdapter 定义了音频数据的生产标准
type AudioAdapter interface {
	Start(callback func([]float32)) error // 启动采集，并通过回调函数外推数据
	Stop() error                          // 停止采集
	GetSourceType() string                // 返回来源类型（如 "MIC", "IOT"）
}
