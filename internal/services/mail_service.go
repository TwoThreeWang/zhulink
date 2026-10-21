package services

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

type MailService struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	Enabled  bool
}

func NewMailService() *MailService {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")

	enabled := host != "" && port != "" && user != "" && pass != "" && from != ""
	if !enabled {
		log.Println("⚠️ MailService disabled: Missing SMTP environment variables.")
	}

	return &MailService{
		Host:     host,
		Port:     port,
		Username: user,
		Password: pass,
		From:     from,
		Enabled:  enabled,
	}
}

func (s *MailService) sendAsync(to []string, subject string, body string) {
	if !s.Enabled {
		return
	}

	// 标题里拼了用户名和文章标题：去掉换行防止注入邮件头，再按 RFC 2047 编码中文和 emoji
	subject = mime.QEncoding.Encode("utf-8", strings.NewReplacer("\r", "", "\n", "").Replace(subject))

	go func() {
		auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
		addr := fmt.Sprintf("%s:%s", s.Host, s.Port)

		mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
		msg := []byte(fmt.Sprintf("To: %s\r\n"+
			"From: ZhuLink 通讯员 <%s>\r\n"+
			"Subject: %s\r\n"+
			"%s\r\n%s", strings.Join(to, ","), s.From, subject, mime, body))

		err := smtp.SendMail(addr, auth, s.From, to, msg)
		if err != nil {
			log.Printf("❌ Failed to send email to %v: %v", to, err)
		} else {
			log.Printf("✅ Email sent to %v: %s", to, subject)
		}
	}()
}

func (s *MailService) parseTemplate(templateName string, data interface{}) (string, error) {
	// Assuming templates are in "web/templates/email/"
	// We might need to adjust path depending on where the binary runs.
	// For dev, verify path.
	path := filepath.Join("web", "templates", "email", templateName)
	t, err := template.ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}
	return buf.String(), nil
}

func (s *MailService) SendWelcomeEmail(email, code string) {
	body, err := s.parseTemplate("welcome.html", map[string]string{
		"Code": code,
	})
	if err != nil {
		log.Printf("Error rendering welcome email: %v", err)
		return
	}
	s.sendAsync([]string{email}, "欢迎加入 ZhuLink，请验证您的邮箱", body)
}

func (s *MailService) SendPasswordResetEmail(email, code string) {
	body, err := s.parseTemplate("reset.html", map[string]string{
		"Code": code,
	})
	if err != nil {
		log.Printf("Error rendering reset email: %v", err)
		return
	}
	s.sendAsync([]string{email}, "[ZhuLink]安全提醒：您正在申请重置 ZhuLink 密码", body)
}

func (s *MailService) SendEmailChangeCode(email, code string) {
	body, err := s.parseTemplate("change_email.html", map[string]string{
		"Code": code,
	})
	if err != nil {
		log.Printf("Error rendering change email: %v", err)
		return
	}
	s.sendAsync([]string{email}, "[ZhuLink]请验证您的新邮箱", body)
}

func (s *MailService) SendCommentNotification(email, activeUser, articleTitle, replyContent, originalContent, postLink string) {
	data := map[string]string{
		"ActiveUser":      activeUser,
		"ArticleTitle":    articleTitle,
		"ReplyContent":    replyContent,
		"OriginalContent": originalContent,
		"PostLink":        postLink,
	}
	body, err := s.parseTemplate("notification.html", data)
	if err != nil {
		log.Printf("Error rendering notification email: %v", err)
		return
	}
	s.sendAsync([]string{email}, "💬 [新回响] "+activeUser+" 回复了你在《"+articleTitle+"》下的评论", body)
}
