package email

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/M-Tier/webform-service/internal/config"
)

type Sender struct {
	cfg *config.Config
}

func NewSender(cfg *config.Config) *Sender {
	return &Sender{cfg: cfg}
}

// ContactForm represents a contact form submission
type ContactForm struct {
	Name    string
	Email   string
	Message string
}

// SendContactEmail sends a contact form submission using the site's email configuration
func (s *Sender) SendContactEmail(form ContactForm, site *config.SiteConfig) error {
	subject := fmt.Sprintf("New contact from %s via %s", form.Name, site.ID)

	body := fmt.Sprintf(`New contact form submission from %s

From: %s
Email: %s

Message:
%s

---
This email was sent from the contact form at %s
`, site.ID, form.Name, form.Email, form.Message, site.ID)

	return s.sendEmail(subject, body, form.Email, site)
}

func (s *Sender) sendEmail(subject, body, replyTo string, site *config.SiteConfig) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	// Build email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", site.SenderName, site.SenderEmail)
	headers["To"] = site.RecipientEmail
	headers["Subject"] = subject
	headers["Reply-To"] = replyTo
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	// Build message
	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// Authenticate and send
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)

	err := smtp.SendMail(
		addr,
		auth,
		site.SenderEmail,
		[]string{site.RecipientEmail},
		[]byte(msg.String()),
	)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
