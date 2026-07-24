// Package otelcolbin downloads, verifies, and caches the pinned
// otelcol-contrib binary used by example validation. The release archive
// for the current OS and architecture is fetched from the
// open-telemetry/opentelemetry-collector-releases GitHub release and
// verified against the SHA-256 checksum pinned in evals/custom/versions.env
// (OTELCOL_CONTRIB_SHA256_<GOOS>_<GOARCH>).
package otelcolbin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EnvPathOverride names the environment variable that, when set, supplies
// the otelcol-contrib binary path directly and skips the download.
const EnvPathOverride = "OTELCOL_CONTRIB_PATH"

// archiveURLFor builds the release archive URL for a version, OS, and
// architecture. It is a package variable so tests can point Fetch at an
// httptest server; production always uses the GitHub release template.
var archiveURLFor = func(version, goos, goarch string) string {
	return fmt.Sprintf(
		"https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v%s/otelcol-contrib_%s_%s_%s.tar.gz",
		version, version, goos, goarch)
}

// ChecksumKey returns the versions.env key carrying the archive checksum
// for the given OS and architecture (for example
// OTELCOL_CONTRIB_SHA256_LINUX_AMD64).
func ChecksumKey(goos, goarch string) string {
	return fmt.Sprintf("OTELCOL_CONTRIB_SHA256_%s_%s", strings.ToUpper(goos), strings.ToUpper(goarch))
}

// Fetch returns the path to a verified otelcol-contrib binary for the
// current OS and architecture. raw is the full key-value content of
// evals/custom/versions.env (Versions.Raw from the harness package). The binary is
// cached under the user cache directory; the cache key includes the version
// and checksum, so pin changes invalidate it.
func Fetch(version string, raw map[string]string) (string, error) {
	if override := os.Getenv(EnvPathOverride); override != "" {
		return override, nil
	}
	if version == "" {
		return "", errors.New("otelcolbin: version is empty")
	}
	key := ChecksumKey(runtime.GOOS, runtime.GOARCH)
	checksum := strings.ToLower(raw[key])
	if checksum == "" {
		return "", fmt.Errorf("otelcolbin: no checksum pinned for %s/%s (versions.env key %s)", runtime.GOOS, runtime.GOARCH, key)
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("otelcolbin: resolve cache dir: %w", err)
	}
	binDir := filepath.Join(cacheRoot, "agent-skills-evals", "otelcol-contrib", version+"-"+checksum[:12])
	binPath := filepath.Join(binDir, "otelcol-contrib")
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	archiveURL := archiveURLFor(version, runtime.GOOS, runtime.GOARCH)
	archive, err := download(archiveURL)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(archive)
	if actual := hex.EncodeToString(sum[:]); actual != checksum {
		return "", fmt.Errorf("otelcolbin: checksum mismatch for %s: got %s, pinned %s", archiveURL, actual, checksum)
	}
	binary, err := extractBinary(archive, "otelcol-contrib")
	if err != nil {
		return "", fmt.Errorf("otelcolbin: %s: %w", archiveURL, err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	// Write to a temp name then rename, so concurrent fetches never see a
	// partial binary.
	tmp, err := os.CreateTemp(binDir, "otelcol-contrib-*")
	if err != nil {
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	if err := os.Rename(tmp.Name(), binPath); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("otelcolbin: %w", err)
	}
	return binPath, nil
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("otelcolbin: download %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("otelcolbin: download %s: HTTP %d", url, response.StatusCode)
	}
	const maxArchiveBytes = 512 << 20 // 512 MiB — well above any otelcol-contrib release archive
	data, err := io.ReadAll(io.LimitReader(response.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("otelcolbin: download %s: %w", url, err)
	}
	if len(data) > maxArchiveBytes {
		return nil, fmt.Errorf("otelcolbin: download %s: archive exceeds %d bytes", url, maxArchiveBytes)
	}
	return data, nil
}

func extractBinary(archive []byte, memberName string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if filepath.Base(header.Name) == memberName && header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("extract %s: %w", memberName, err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("member %s not found in archive", memberName)
}
