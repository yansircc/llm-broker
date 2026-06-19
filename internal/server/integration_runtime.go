package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/email"
	"github.com/yansircc/llm-broker/internal/payments"
)

func integrationConfig(integration *domain.Integration) map[string]string {
	out := map[string]string{}
	if integration == nil || strings.TrimSpace(integration.ConfigJSON) == "" {
		return out
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(integration.ConfigJSON), &raw); err != nil {
		return out
	}
	for k, v := range raw {
		out[k] = strings.TrimSpace(fmt.Sprint(v))
	}
	return out
}

func (s *Server) decryptedIntegrationConfig(ctx context.Context, integration *domain.Integration) (payments.Config, error) {
	cfg := payments.Config{Public: integrationConfig(integration), Secrets: map[string]string{}}
	if s == nil || s.settings == nil {
		return cfg, nil
	}
	secrets, err := s.settings.DecryptSecrets(integration)
	if err != nil {
		return cfg, err
	}
	cfg.Secrets = secrets
	return cfg, nil
}

func (s *Server) enabledIntegrations(ctx context.Context, kind, provider string) ([]*domain.Integration, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.ListEnabledIntegrations(ctx, kind, provider)
}

func (s *Server) recordIntegrationEvent(ctx context.Context, integration *domain.Integration, eventType string, success bool, errCode string, payload map[string]any) {
	if s == nil || s.store == nil || integration == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	raw, _ := json.Marshal(payload)
	_ = s.store.SaveIntegrationEvent(ctx, &domain.IntegrationEvent{
		ID:                  uuid.NewString(),
		IntegrationID:       integration.ID,
		Kind:                integration.Kind,
		EventType:           eventType,
		Success:             success,
		ErrorCode:           errCode,
		RedactedPayloadJSON: string(raw),
		CreatedAt:           time.Now().UTC(),
	})
}

func (s *Server) sendEmail(ctx context.Context, msg email.Message) error {
	integrations, err := s.enabledIntegrations(ctx, "email", "")
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		if s.emailSender == nil {
			return email.StdoutSender{}.Send(ctx, msg)
		}
		return s.emailSender.Send(ctx, msg)
	}
	var lastErr error
	for _, integration := range integrations {
		cfg, err := s.decryptedIntegrationConfig(ctx, integration)
		if err != nil {
			lastErr = err
			s.recordIntegrationEvent(ctx, integration, "send", false, "decrypt_failed", nil)
			continue
		}
		sender, err := emailSenderFromIntegration(integration.Provider, cfg)
		if err != nil {
			lastErr = err
			s.recordIntegrationEvent(ctx, integration, "send", false, "unsupported_provider", nil)
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			lastErr = err
			s.recordIntegrationEvent(ctx, integration, "send", false, "send_failed", map[string]any{"to_hash": emailAddressHash(msg.To), "subject": msg.Subject})
			slog.Warn("email provider failed", "integration_id", integration.ID, "provider", integration.Provider, "error", err)
			continue
		}
		s.recordIntegrationEvent(ctx, integration, "send", true, "", map[string]any{"to_hash": emailAddressHash(msg.To), "subject": msg.Subject})
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no email provider configured")
	}
	return lastErr
}

func emailSenderFromIntegration(provider string, cfg payments.Config) (email.Sender, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "smtp":
		return email.SMTPSender{
			Addr:     cfg.Public["addr"],
			Username: cfg.Public["username"],
			Password: cfg.Secrets["password"],
			From:     cfg.Public["from"],
		}, nil
	case "resend":
		return email.ResendSender{
			APIKey: cfg.Secrets["api_key"],
			From:   cfg.Public["from"],
		}, nil
	default:
		return nil, fmt.Errorf("unsupported email provider %q", provider)
	}
}

func emailAddressHash(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Server) turnstileConfig(ctx context.Context) (bool, string, string) {
	integrations, err := s.enabledIntegrations(ctx, "security", "turnstile")
	if err == nil {
		for _, integration := range integrations {
			cfg := integrationConfig(integration)
			secret := ""
			if s.settings != nil {
				if secrets, err := s.settings.DecryptSecrets(integration); err == nil {
					secret = secrets["secret_key"]
				}
			}
			return integration.Enabled, cfg["site_key"], secret
		}
	}
	if s != nil && s.cfg != nil && s.cfg.TurnstileEnabled {
		return true, s.cfg.TurnstileSiteKey, s.cfg.TurnstileSecretKey
	}
	return false, "", ""
}
