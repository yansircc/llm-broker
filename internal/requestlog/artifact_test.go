package requestlog

import "testing"

func TestParseBlobMode(t *testing.T) {
	cases := []struct {
		input string
		want  BlobMode
	}{
		{"", BlobModeErrors},
		{"errors", BlobModeErrors},
		{"Errors", BlobModeErrors},
		{"  errors  ", BlobModeErrors},
		{"error", BlobModeErrors},
		{"off", BlobModeOff},
		{"OFF", BlobModeOff},
		{"false", BlobModeOff},
		{"0", BlobModeOff},
		{"no", BlobModeOff},
		{"none", BlobModeOff},
		{"all", BlobModeAll},
		{"ALL", BlobModeAll},
		{"true", BlobModeAll},
		{"1", BlobModeAll},
		{"yes", BlobModeAll},
		{"garbage", BlobModeErrors},
	}
	for _, c := range cases {
		if got := ParseBlobMode(c.input); got != c.want {
			t.Fatalf("ParseBlobMode(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestBlobModeShouldWrite(t *testing.T) {
	cases := []struct {
		mode   BlobMode
		status string
		want   bool
	}{
		{BlobModeOff, "ok", false},
		{BlobModeOff, "upstream_429", false},
		{BlobModeOff, "transport_error", false},
		{BlobModeAll, "ok", true},
		{BlobModeAll, "upstream_429", true},
		{BlobModeAll, "stream_incomplete", true},
		{BlobModeErrors, "ok", false},
		{BlobModeErrors, "  ok  ", false},
		{BlobModeErrors, "upstream_429", true},
		{BlobModeErrors, "transport_error", true},
		{BlobModeErrors, "stream_incomplete", true},
		{BlobModeErrors, "validation_400", true},
		{BlobModeErrors, "", true},
	}
	for _, c := range cases {
		if got := c.mode.ShouldWrite(c.status); got != c.want {
			t.Fatalf("(%q).ShouldWrite(%q) = %v, want %v", c.mode, c.status, got, c.want)
		}
	}
}

func TestResolveBlobDirRespectsMode(t *testing.T) {
	if dir := ResolveBlobDir("/tmp/x/data.db", BlobModeOff); dir != "" {
		t.Fatalf("BlobModeOff should disable; got %q", dir)
	}
	if dir := ResolveBlobDir("/tmp/x/data.db", BlobModeErrors); dir != "/tmp/x/request-log-blobs" {
		t.Fatalf("BlobModeErrors path = %q", dir)
	}
	if dir := ResolveBlobDir("/tmp/x/data.db", BlobModeAll); dir != "/tmp/x/request-log-blobs" {
		t.Fatalf("BlobModeAll path = %q", dir)
	}
	if dir := ResolveBlobDir(":memory:", BlobModeAll); dir != "" {
		t.Fatalf(":memory: should disable; got %q", dir)
	}
	if dir := ResolveBlobDir("", BlobModeAll); dir != "" {
		t.Fatalf("empty dbPath should disable; got %q", dir)
	}
}
