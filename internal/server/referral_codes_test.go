package server

import "testing"

func TestGenerateReferralCodeFormat(t *testing.T) {
	for range 128 {
		code, err := generateReferralCode()
		if err != nil {
			t.Fatalf("generateReferralCode: %v", err)
		}
		if len(code) != referralCodeLength {
			t.Fatalf("len(code) = %d, want %d for %q", len(code), referralCodeLength, code)
		}
		for _, r := range code {
			if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				t.Fatalf("code %q contains non-alphanumeric rune %q", code, r)
			}
		}
	}
}
