package zpay

import "testing"

func TestSignExcludesEmptySignAndSignTypeThenSortsASCII(t *testing.T) {
	params := map[string]string{
		"b":         "2",
		"a":         "1",
		"A":         "0",
		"empty":     "",
		"sign":      "bad",
		"sign_type": "MD5",
	}

	got := Sign(params, "secret")
	want := "998ef127a6d226014c43f6cff1b32bee"
	if got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	params := map[string]string{
		"pid":          "1001",
		"money":        "9.90",
		"out_trade_no": "order-1",
	}
	params["sign"] = Sign(params, "secret")

	if !Verify(params, "secret") {
		t.Fatal("Verify() rejected valid signature")
	}

	params["money"] = "0.01"
	if Verify(params, "secret") {
		t.Fatal("Verify() accepted tampered parameters")
	}
}
