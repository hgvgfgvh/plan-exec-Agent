package communicationDomain

import (
	"AgentTest/behavior/skill"
	"context"
	"fmt"

	"log"

	"gopkg.in/gomail.v2"
)

// EmailSkill 对应 Send_Email_With_Attachments 技能
type EmailSkill struct {
	// SMTP 服务器配置
	SMTPServer string
	SMTPPort   int
	Sender     string
	AuthCode   string // 注意：这里填的是邮箱设置里的“授权码”，不是登录密码
}

func NewEmailSkill() *EmailSkill {
	return &EmailSkill{
		// 以 QQ 邮箱为例，如果是 Gmail 或网易请修改对应的服务器
		SMTPServer: "smtp.qq.com",
		SMTPPort:   465,
		Sender:     "2563726816@qq.com",
		AuthCode:   "bcceuwtceqafebhc",
	}
}

func (s *EmailSkill) Name() string { return "Send_Email_With_Attachments" }

func (s *EmailSkill) Description() string {
	return "通过 SMTP 发送带附件的邮件，支持发送生成的 Word 或 PPT 文件"
}

func (s *EmailSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 预检
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. 参数解析 (recipient, subject, body, attachments)
	if len(args) < 4 {
		return nil, fmt.Errorf("Email 技能需要 4 个参数: 收件人, 主题, 正文, 附件列表")
	}

	recipient, _ := args[0].(string)
	subject, _ := args[1].(string)
	body, _ := args[2].(string)

	// 附件列表处理：大脑通常传过来的是 []interface{} 或 []string
	var attachmentPaths []string
	if rawAttachments, ok := args[3].([]interface{}); ok {
		for _, v := range rawAttachments {
			if path, ok := v.(string); ok {
				attachmentPaths = append(attachmentPaths, path)
			}
		}
	} else if singlePath, ok := args[3].(string); ok {
		// 容错处理：如果只传了一个字符串路径而非数组
		attachmentPaths = append(attachmentPaths, singlePath)
	}

	fmt.Printf("📧 准备发送邮件至: [%s], 附件数量: %d\n", recipient, len(attachmentPaths))

	// 3. 构造邮件消息
	m := gomail.NewMessage()
	m.SetHeader("From", s.Sender)
	m.SetHeader("To", recipient)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body) // 支持 HTML 格式，让正文更好看

	// 挂载附件
	for _, path := range attachmentPaths {
		if path != "" {
			m.Attach(path)
			fmt.Printf("📎 已挂载附件: %s\n", path)
		}
	}

	// 4. 发送邮件
	d := gomail.NewDialer(s.SMTPServer, s.SMTPPort, s.Sender, s.AuthCode)

	// 执行发送
	if err := d.DialAndSend(m); err != nil {
		log.Printf("❌ 邮件发送失败: %v\n", err)
		return []interface{}{"fail"}, err
	}

	fmt.Println("✅ 邮件已成功送达！")
	return []interface{}{"success"}, nil
}

func init() {
	skill.GlobalManager.Regist(NewEmailSkill())
}
