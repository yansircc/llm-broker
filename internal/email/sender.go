package email

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type StdoutSender struct{}

func (StdoutSender) Send(_ context.Context, msg Message) error {
	slog.Info("email verification message", "to", msg.To, "subject", msg.Subject)
	return nil
}

type SMTPSender struct {
	Addr     string
	Username string
	Password string
	From     string
}

func (s SMTPSender) Send(ctx context.Context, msg Message) error {
	if s.Addr == "" || s.From == "" {
		return fmt.Errorf("smtp sender is not configured")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	var body bytes.Buffer
	body.WriteString("From: " + s.From + "\r\n")
	body.WriteString("To: " + msg.To + "\r\n")
	body.WriteString("Subject: " + msg.Subject + "\r\n")
	body.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	body.WriteString("\r\n")
	body.WriteString(msg.Text)
	auth := smtp.PlainAuth("", s.Username, s.Password, strings.Split(s.Addr, ":")[0])
	if s.Username == "" && s.Password == "" {
		auth = nil
	}
	return smtp.SendMail(s.Addr, auth, s.From, []string{msg.To}, body.Bytes())
}

func VerificationMessage(to, link string) Message {
	return Message{
		To:      to,
		Subject: "Verify your email",
		Text:    "Open this link to verify your email:\n\n" + link + "\n\nThis link expires in 1 hour.",
	}
}
