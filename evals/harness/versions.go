package harness

import (
	"fmt"
	"os"
	"strings"
)

// Versions carries the pinned tool versions from evals/versions.env. The pins
// change only through PRs, which run the full matrix (R19).
type Versions struct {
	// ClaudeCodeVersion is the pinned Claude Code CLI version
	// (CLAUDE_CODE_VERSION).
	ClaudeCodeVersion string
	// EvalModel is the pinned model ID passed to --model (EVAL_MODEL).
	EvalModel string
	// OtelcolContribVersion is the pinned otelcol-contrib version shared
	// with example validation (OTELCOL_CONTRIB_VERSION).
	OtelcolContribVersion string
	// OtelGoCoreVersion is the pinned version of the stable OpenTelemetry Go
	// modules (OTEL_GO_CORE_VERSION), used to compile complete Go SDK code
	// blocks.
	OtelGoCoreVersion string
	// OtelGoLogVersion is the pinned version of the pre-release
	// OpenTelemetry Go log modules (OTEL_GO_LOG_VERSION).
	OtelGoLogVersion string
	// Raw carries every key in the file, including checksums added by
	// later units.
	Raw map[string]string
}

// LoadVersions parses a KEY=VALUE env file (comments with #, blank lines
// ignored) into a Versions.
func LoadVersions(path string) (*Versions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("versions: read %s: %w", path, err)
	}
	raw := map[string]string{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("versions: %s:%d: not KEY=VALUE: %q", path, i+1, line)
		}
		raw[strings.TrimSpace(key)] = parseEnvValue(value)
	}
	v := &Versions{
		ClaudeCodeVersion:     raw["CLAUDE_CODE_VERSION"],
		EvalModel:             raw["EVAL_MODEL"],
		OtelcolContribVersion: raw["OTELCOL_CONTRIB_VERSION"],
		OtelGoCoreVersion:     raw["OTEL_GO_CORE_VERSION"],
		OtelGoLogVersion:      raw["OTEL_GO_LOG_VERSION"],
		Raw:                   raw,
	}
	if v.ClaudeCodeVersion == "" || v.EvalModel == "" || v.OtelcolContribVersion == "" {
		return nil, fmt.Errorf("versions: %s must pin CLAUDE_CODE_VERSION, EVAL_MODEL, and OTELCOL_CONTRIB_VERSION", path)
	}
	if v.OtelGoCoreVersion == "" || v.OtelGoLogVersion == "" {
		return nil, fmt.Errorf("versions: %s must pin OTEL_GO_CORE_VERSION and OTEL_GO_LOG_VERSION", path)
	}
	return v, nil
}

// parseEnvValue resolves a raw KEY=VALUE right-hand side the way `source`ing
// the file in POSIX sh would: a quoted value keeps its interior verbatim, and
// an unquoted value ends at a whitespace-introduced inline comment. This keeps
// the Go loader and the shell `source evals/versions.env` in the CI workflows
// from diverging on the same file (a divergence could, for example, leave a
// version pin carrying a trailing "# comment" only in Go).
func parseEnvValue(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, "\t#"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}
