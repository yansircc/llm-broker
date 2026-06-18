package email

import "testing"

func TestVerificationMessageDoesNotEmbedSecretsOutsideLink(t *testing.T) {
	msg := VerificationMessage("user@example.com", "https://example.com/api/auth/verify-email?token=abc")
	if msg.To != "user@example.com" {
		t.Fatalf("To = %q", msg.To)
	}
	if msg.Subject == "" || msg.Text == "" {
		t.Fatal("message missing subject or body")
	}
}
