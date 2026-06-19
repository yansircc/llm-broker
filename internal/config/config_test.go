package config

import (
	"strings"
	"testing"
)

func TestValidateAcceptsExternalServicesWithoutSiteURL(t *testing.T) {
	cfg := Config{
		EncryptionKey:      "enc",
		StaticToken:        "tok",
		SMTPAddr:           "smtp.example.com:587",
		SMTPFrom:           "relay@example.com",
		ZPayPID:            "1001",
		ZPayKey:            "secret",
		BackgroundJobsMode: "all",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}

func TestValidateAcceptsCanonicalHTTPSiteURL(t *testing.T) {
	cfg := Config{
		EncryptionKey:      "enc",
		StaticToken:        "tok",
		SiteURL:            "https://relay.example.com",
		SMTPAddr:           "smtp.example.com:587",
		SMTPFrom:           "relay@example.com",
		BackgroundJobsMode: "all",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}

func TestValidateRejectsInvalidSiteURLWhenConfigured(t *testing.T) {
	cfg := Config{
		EncryptionKey:      "enc",
		StaticToken:        "tok",
		SiteURL:            "not-a-url",
		BackgroundJobsMode: "all",
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "SITE_URL") {
		t.Fatalf("Validate() = %v, want SITE_URL error", err)
	}
}

func TestValidateRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	cfg := Config{
		EncryptionKey:      "enc",
		StaticToken:        "tok",
		BackgroundJobsMode: "all",
		TrustedProxyCIDRs:  []string{"not-a-cidr"},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("Validate() = %v, want TRUSTED_PROXY_CIDRS error", err)
	}
}
