package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresSiteURLWhenExternalURLSendersEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "smtp",
			cfg: Config{
				EncryptionKey: "enc",
				StaticToken:   "tok",
				SMTPAddr:      "smtp.example.com:587",
				SMTPFrom:      "relay@example.com",
			},
		},
		{
			name: "zpay",
			cfg: Config{
				EncryptionKey: "enc",
				StaticToken:   "tok",
				ZPayPID:       "1001",
				ZPayKey:       "secret",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.BackgroundJobsMode = "all"
			err := tc.cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "SITE_URL") {
				t.Fatalf("Validate() = %v, want SITE_URL error", err)
			}
		})
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
