package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadDotEnv loads KEY=VALUE pairs from the repository-root .env file into the
// process environment for local runs, so a developer can keep ANTHROPIC_API_KEY
// in .env instead of prefixing every go test command with it.
//
// It never overwrites a variable already set in the environment, so a value
// passed on the command line (or by CI) always wins. A missing file is not an
// error — CI has no .env and relies on real environment variables. It returns
// the sorted names of the variables it set, so callers can log what was loaded
// without ever printing the values.
//
// Parsing matches LoadVersions: # comments and blank lines are ignored, and an
// unquoted value ends at a whitespace-introduced inline comment.
func LoadDotEnv() ([]string, error) {
	root, err := repoRootFromWD()
	if err != nil {
		// Without a repo root there is nothing to load; that is not fatal.
		return nil, nil
	}
	set, err := loadDotEnvFile(filepath.Join(root, ".env"))
	if err != nil {
		return nil, err
	}
	sort.Strings(set)
	return set, nil
}

// loadDotEnvFile loads one .env file, setting only variables not already
// present in the environment. A missing file is a no-op.
func loadDotEnvFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("dotenv: read %s: %w", path, err)
	}
	var set []string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("dotenv: %s:%d: not KEY=VALUE: %q", path, i+1, line)
		}
		key = strings.TrimSpace(key)
		if _, present := os.LookupEnv(key); present {
			continue
		}
		if err := os.Setenv(key, parseEnvValue(value)); err != nil {
			return nil, fmt.Errorf("dotenv: set %s: %w", key, err)
		}
		set = append(set, key)
	}
	return set, nil
}

// repoRootFromWD walks up from the working directory to the repository root,
// the nearest ancestor containing a skills/ directory.
func repoRootFromWD() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "skills")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("dotenv: no skills/ directory found above the working directory")
		}
		dir = parent
	}
}
