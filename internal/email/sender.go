package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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

type ResendSender struct {
	APIKey     string
	From       string
	HTTPClient *http.Client
}

func (s ResendSender) Send(ctx context.Context, msg Message) error {
	if s.APIKey == "" || s.From == "" {
		return fmt.Errorf("resend sender is not configured")
	}
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(map[string]any{
		"from":    s.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("resend send: %s", resp.Status)
	}
	return nil
}

func VerificationMessage(to, link string) Message {
	return Message{
		To:      to,
		Subject: "Verify your email",
		Text:    "Open this link to verify your email:\n\n" + link + "\n\nThis link expires in 1 hour.",
	}
}
