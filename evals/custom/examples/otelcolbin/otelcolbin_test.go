package otelcolbin

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestChecksumKey(t *testing.T) {
	if got := ChecksumKey("linux", "amd64"); got != "OTELCOL_CONTRIB_SHA256_LINUX_AMD64" {
		t.Errorf("ChecksumKey = %q", got)
	}
	if got := ChecksumKey("darwin", "arm64"); got != "OTELCOL_CONTRIB_SHA256_DARWIN_ARM64" {
		t.Errorf("ChecksumKey = %q", got)
	}
}

func TestFetchRequiresChecksum(t *testing.T) {
	_, err := Fetch("0.156.0", map[string]string{})
	if err == nil {
		t.Fatalf("expected error for missing checksum")
	}
	if !strings.Contains(err.Error(), ChecksumKey(runtime.GOOS, runtime.GOARCH)) {
		t.Errorf("error does not name the missing key: %v", err)
	}
}

func TestFetchChecksumMismatch(t *testing.T) {
	// A temp cache dir keeps the test off any real cache, and forces the
	// download path (the wrong-checksum cache key never exists on disk).
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	served := []byte("these bytes are not the pinned archive")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(served)
	}))
	defer server.Close()

	original := archiveURLFor
	archiveURLFor = func(_, _, _ string) string { return server.URL }
	defer func() { archiveURLFor = original }()

	// A checksum that does not match the served bytes.
	raw := map[string]string{
		ChecksumKey(runtime.GOOS, runtime.GOARCH): strings.Repeat("0", 64),
	}
	_, err := Fetch("0.156.0", raw)
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error is not a checksum mismatch: %v", err)
	}
}

func TestFetchPathOverride(t *testing.T) {
	t.Setenv(EnvPathOverride, "/usr/local/bin/otelcol-contrib")
	path, err := Fetch("0.156.0", nil)
	if err != nil {
		t.Fatalf("Fetch with override: %v", err)
	}
	if path != "/usr/local/bin/otelcol-contrib" {
		t.Errorf("path = %q", path)
	}
}
