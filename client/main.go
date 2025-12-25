package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EmailClient struct {
	Server   string
	Port     int
	Username string
	Password string
}

type Email struct {
	From        string
	To          []string
	Subject     string
	Body        string
	Attachments []string
}

func NewEmailClient(server string, port int, username, password string) *EmailClient {
	return &EmailClient{
		Server:   server,
		Port:     port,
		Username: username,
		Password: password,
	}
}

func (c *EmailClient) SendEmail(email *Email) error {
	// 创建邮件内容
	var buf bytes.Buffer

	// 邮件头
	buf.WriteString(fmt.Sprintf("From: %s\r\n", email.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.To, ",")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))

	// 生成边界
	boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	buf.WriteString("\r\n")

	// 邮件正文部分
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(email.Body)
	buf.WriteString("\r\n")

	// 处理附件
	for _, filename := range email.Attachments {
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("读取附件文件失败: %v", err)
		}

		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString("Content-Type: application/octet-stream\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%s\r\n", filepath.Base(filename)))
		buf.WriteString("\r\n")

		// Base64编码附件内容
		encoded := base64.StdEncoding.EncodeToString(content)
		// 按76字符分行
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			buf.WriteString(encoded[i:end] + "\r\n")
		}
		buf.WriteString("\r\n")
	}

	// 结束边界
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	//连接服务器，不使用SMTP认证，也不使用 tls
	conn, err := net.Dial("tcp", "127.0.0.1:25")
	if err != nil {
		fmt.Printf("✗ 连接服务器失败,请检查主机地址和端口是否正确！错误原因:-> %v\n", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, c.Server)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %v", err)
	}
	defer client.Quit()

	// 设置发件人
	if err = client.Mail(email.From); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}

	// 设置收件人
	for _, recipient := range email.To {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %v", err)
		}
	}

	// 发送邮件内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("创建数据写入器失败: %v", err)
	}

	_, err = writer.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("关闭数据写入器失败: %v", err)
	}

	return nil
}

// 发送有附件的邮件,完成邮件的整体保存和附件的独立保存。
func main() {

	client := NewEmailClient("127.0.0.1", 25, "your_email@gmail.com", "your_password")

	// 创建邮件
	email := &Email{
		From:    "your_email@gmail.com",
		To:      []string{"recipient1@example.com", "recipient2@example.com"},
		Subject: "测试邮件附件",
		Body:    "这是一封包含附件的测试邮件。\n\n请查收附件内容。\n\n谢谢！",
		Attachments: []string{
			"text.txt",
			"img.png",
		},
	}

	// 发送邮件
	fmt.Println("正在发送邮件...")
	if err := client.SendEmail(email); err != nil {
		fmt.Printf("发送邮件失败: %v\n", err)
		return
	}

	fmt.Println("邮件发送成功！")
}
