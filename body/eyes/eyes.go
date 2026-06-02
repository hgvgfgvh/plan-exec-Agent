package eyes

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

// Eyes 定义了一个独立的视觉通道（如 PC屏幕、IoT摄像头等）
type Eyes struct {
	Name        string        // 通道名称
	mu          sync.RWMutex  // 读写锁，保证并发安全
	current     image.Image   // 当前视野引用
	currentHash uint64        // 当前 AHash 指纹
	history     []image.Image // 历史队列 (FIFO)
	maxHistory  int           // 队列容量
}

// NewEyes 创建一个新的视觉通道实例
func NewEyes(name string) *Eyes {
	return &Eyes{
		Name:       name,
		history:    make([]image.Image, 0),
		maxHistory: 5,
	}
}

// See 更新当前视野
func (e *Eyes) See(img image.Image) {
	if img == nil {
		return
	}
	newHash := e.CalculateAHash(img)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current != nil {
		e.history = append(e.history, e.current)
	}
	if len(e.history) > e.maxHistory {
		e.history = e.history[1:]
	}

	e.current = img
	e.currentHash = newHash
}

// GetCurrentHash 获取当前图像的哈希指纹

func (e *Eyes) GetCurrentHash() uint64 {

	e.mu.RLock()

	defer e.mu.RUnlock()

	return e.currentHash

}

// CalculateAHash 计算均值哈希 (内部算法不变)
func (e *Eyes) CalculateAHash(img image.Image) uint64 {
	smallImg := imaging.Resize(img, 8, 8, imaging.NearestNeighbor)
	grayImg := imaging.Grayscale(smallImg)

	var sum uint64
	pixels := make([]uint8, 64)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			r, _, _, _ := grayImg.At(x, y).RGBA()
			val := uint8(r >> 8)
			pixels[y*8+x] = val
			sum += uint64(val)
		}
	}
	avg := uint8(sum / 64)

	var hash uint64
	for i := 0; i < 64; i++ {
		if pixels[i] >= avg {
			hash |= (1 << uint(i))
		}
	}
	return hash
}

// IsSignificantlyDifferent 汉明距离对比
func (e *Eyes) IsSignificantlyDifferent(img image.Image, threshold int) bool {
	newHash := e.CalculateAHash(img)
	e.mu.RLock()
	currHash := e.currentHash
	e.mu.RUnlock()

	distance := 0
	res := currHash ^ newHash
	for res > 0 {
		res &= (res - 1)
		distance++
	}
	return distance > threshold
}

// GetSnapshot 获取快照
func (e *Eyes) GetSnapshot() image.Image {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current
}

// GetProcessedCopy 处理并导出副本
func (e *Eyes) GetProcessedCopy(width, height int, gray bool) (string, error) {
	e.mu.RLock()
	if e.current == nil {
		e.mu.RUnlock()
		return "", fmt.Errorf("[%s] current view is empty", e.Name)
	}
	imgCopy := imaging.Clone(e.current)
	e.mu.RUnlock()

	if width > 0 && height > 0 {
		imgCopy = imaging.Resize(imgCopy, width, height, imaging.Lanczos)
	}
	if gray {
		imgCopy = imaging.Grayscale(imgCopy)
	}

	tempDir := os.TempDir()
	fileName := fmt.Sprintf("eye_%s_%d.png", e.Name, time.Now().UnixNano())
	filePath := filepath.Join(tempDir, fileName)

	err := imaging.Save(imgCopy, filePath)
	return filePath, err
}

// SetMaxHistory 调整记忆长度
func (e *Eyes) SetMaxHistory(size int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.maxHistory = size
}

// abs 辅助函数
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ==================== 变化检测 ====================

// 参数: threshold - 变化像素比例阈值

// 返回: bool - 是否检测到显著变化

// 注意: 此方法效率较低，推荐使用IsSignificantlyDifferent

func (e *Eyes) HasChanged(threshold float64) bool {

	e.mu.RLock()

	defer e.mu.RUnlock()

	// 边界检查

	if len(e.history) == 0 || e.current == nil {

		return true

	}

	// 获取最近的历史图像

	prev := e.history[len(e.history)-1]

	curr := e.current

	// 尺寸变化直接认为是变化

	if prev.Bounds() != curr.Bounds() {

		return true

	}

	// 采样比较像素差异

	var diff uint64

	b := curr.Bounds()

	step := 10 // 采样步长，提高效率

	for y := b.Min.Y; y < b.Max.Y; y += step {

		for x := b.Min.X; x < b.Max.X; x += step {

			r1, g1, b1, _ := prev.At(x, y).RGBA()

			r2, g2, b2, _ := curr.At(x, y).RGBA()

			// 计算颜色差异

			colorDiff := abs(int(r1)-int(r2)) +
				abs(int(g1)-int(g2)) +
				abs(int(b1)-int(b2))

			if colorDiff > 1000 { // 经验阈值

				diff++

			}

		}

	}

	// 计算变化比例

	totalSamples := (b.Dx() / step) * (b.Dy() / step)

	changeRatio := float64(diff) / float64(totalSamples)

	return changeRatio > threshold

}
