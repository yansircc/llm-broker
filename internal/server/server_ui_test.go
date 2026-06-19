package server

import "testing"

func TestReservedUIPathKeepsAdminAPIsAndOldAdminUIOutOfSPAFallback(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/console/dashboard", want: false},
		{path: "/console/accounts/acct-1", want: false},
		{path: "/console/login", want: true},
		{path: "/dashboard", want: true},
		{path: "/accounts/acct-1", want: true},
		{path: "/users/u-1", want: true},
		{path: "/activity", want: true},
		{path: "/migrations", want: true},
		{path: "/cells/cell-1", want: true},
		{path: "/admin-billing/orders", want: true},
		{path: "/login", want: true},
		{path: "/add-account/codex", want: true},
		{path: "/admin/dashboard", want: true},
		{path: "/api/me", want: true},
		{path: "/ready", want: true},
	}
	for _, tt := range tests {
		if got := isReservedUIPath(tt.path); got != tt.want {
			t.Fatalf("isReservedUIPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
