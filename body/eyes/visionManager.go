package eyes

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // 注册相机驱动
	//"gocv.io/x/gocv"
	_ "github.com/pion/mediadevices/pkg/frame" // 必须导入这个包处理格式
)

// VisionManager 视觉总管
type VisionManager struct {
	mu        sync.RWMutex
	Viewports map[string]*Eyes // 不同的 Key 代表不同的视角（Viewport）
	cancel    context.CancelFunc
}

var (
	mgmt *VisionManager
	once sync.Once
)

// GetManager 获取视觉单例
func GetManager() *VisionManager {
	return mgmt
}

func init() {
	// 1. 初始化管理对象
	mgmt = &VisionManager{
		Viewports: map[string]*Eyes{
			"PC":     NewEyes("PC"),
			"Camera": NewEyes("Camera"),
		},
	}

	// 2. 启动自动截屏注入逻辑
	// 使用 context 以便后续可能的停机管理
	ctx, cancel := context.WithCancel(context.Background())
	mgmt.cancel = cancel

	// 启动后台扫描
	go mgmt.autoCaptureLoop(ctx)
	go mgmt.autoCameraLoop(ctx) // 启动摄像头扫描
}

// autoCaptureLoop 核心注入逻辑：每秒捕获屏幕并更新到对应的 Eyes
func (m *VisionManager) autoCaptureLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 预先获取 PC 视野引用，减少循环内的 Map 查找开销
	pcEye := m.Viewports["PC"]

	fmt.Println("👁️  [VisionSystem] 屏幕自动扫描已启动 (1Hz)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 捕获主显示器 (0)
			img, err := screenshot.CaptureDisplay(0)
			if err != nil {
				// 这里建议仅打印或记录日志，不要中断循环
				continue
			}

			// 注入数据：See 方法内部会处理 RWMutex 锁、AHash 计算和历史队列
			pcEye.See(img)
		}
	}
}
func (m *VisionManager) autoCameraLoop(ctx context.Context) {
	cameraEye := m.Viewports["Camera"]
	width, height := 1280, 720
	frameSize := width * height * 3

	// 1. 动态获取设备名称 (增强适配性)
	rawName := m.findVideoDevice()
	if rawName == "" {
		fmt.Println("❌ 无法识别到有效的摄像头设备，请检查硬件连接或权限设置")
		return
	}
	// FFmpeg DirectShow 输入要求格式为 "video=设备名"
	deviceName := "video=" + rawName
	fmt.Printf("🚀 正在接入动态识别的摄像头: [%s]\n", deviceName)

	// 2. 构造命令
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-f", "dshow",
		"-rtbufsize", "500M",
		"-i", deviceName,
		"-vcodec", "rawvideo",
		"-pix_fmt", "rgb24",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-r", "0.5", // 2秒1帧
		"-f", "image2pipe",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("❌ 无法创建 Stdout 管道: %v\n", err)
		return
	}

	// 捕获错误日志以供调试
	stderr, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Error") || strings.Contains(line, "fail") {
				fmt.Printf("🎬 [FFmpeg Error]: %s\n", line)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ FFmpeg 启动失败: %v\n", err)
		return
	}

	fmt.Println("📷 摄像头 [FFmpeg] 管道已就绪")

	// 3. 核心循环：使用预分配的 Buffer 避免内存泄漏/频繁 GC
	go func() {
		defer cmd.Process.Kill()

		// 【内存优化】在循环外只分配一次内存
		frameBuffer := make([]byte, frameSize)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 使用 ReadFull 填满预分配的 frameBuffer
				_, err := io.ReadFull(stdout, frameBuffer)
				if err != nil {
					if err != io.EOF {
						fmt.Printf("⚠ 摄像头流中断: %v\n", err)
					}
					return
				}

				// 将 Buffer 转换为图像并注入
				// 注意：bytesToImage 会创建新的 image 对象，这是必要的，
				// 因为 See 方法会将图片存入历史队列(history)，必须保证图片数据的独立性。
				img := bytesToImage(frameBuffer, width, height)
				cameraEye.See(img)
			}
		}
	}()

	<-ctx.Done()
}

// findVideoDevice 自动扫描系统中的第一个视频设备名称
func (m *VisionManager) findVideoDevice() string {
	fmt.Println("🔍 [Debug] 正在启动增强型设备扫描...")

	cmd := exec.Command("ffmpeg", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if len(output) == 0 {
		return ""
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 1. 核心改进：只要这一行包含 "(video)"，它就是我们要找的视频设备名所在行
		// 这样可以绕过缺失的 "DirectShow video devices" 标题行
		if strings.Contains(line, "(video)") {

			// 2. 排除别名行（虽然通常 (video) 只出现在友好名称行）
			if strings.Contains(line, "Alternative name") {
				continue
			}

			// 3. 提取引号内容
			// 使用 LastIndex 处理可能存在的多个引号冲突
			firstQuote := strings.Index(line, "\"")
			lastQuote := strings.LastIndex(line, "\"")

			if firstQuote != -1 && lastQuote != -1 && firstQuote < lastQuote {
				deviceName := line[firstQuote+1 : lastQuote]

				// 清理掉可能存在的不可见字符
				deviceName = strings.TrimSpace(deviceName)

				if deviceName != "" {
					fmt.Printf("🎯 [Debug] 捕获成功: [%s]\n", deviceName)
					return deviceName
				}
			}
		}
	}

	// 4. 备选方案：如果上面的逻辑依然失败，尝试暴力搜索包含 [dshow @ ... ] 且带引号的第一行
	fmt.Println("🚧 [Debug] 关键字匹配未果，尝试暴力降级解析...")
	for _, line := range lines {
		if strings.Contains(line, "[dshow @") && strings.Contains(line, "\"") && !strings.Contains(line, "Alternative name") {
			// 排除掉音频设备（通常包含 audio 或类似麦克风字样）
			if strings.Contains(line, "audio") || strings.Contains(line, "麦克风") {
				continue
			}
			firstQuote := strings.Index(line, "\"")
			lastQuote := strings.LastIndex(line, "\"")
			return line[firstQuote+1 : lastQuote]
		}
	}

	fmt.Println("❌ [Debug] 所有解析逻辑均未找到视频设备")
	return ""
}

// 辅助函数：将 RGB24 原始数据转为图像对象
func bytesToImage(data []byte, w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 3
			pixIdx := (y*w + x) * 4
			img.Pix[pixIdx] = data[idx]     // R
			img.Pix[pixIdx+1] = data[idx+1] // G
			img.Pix[pixIdx+2] = data[idx+2] // B
			img.Pix[pixIdx+3] = 255         // A
		}
	}
	return img
}

// StopVision 停止所有视觉扫描（优雅关闭接口）
func (m *VisionManager) StopVision() {
	if m.cancel != nil {
		m.cancel()
	}
}
