// Command validate-examples extracts every fenced code block from the skill
// markdown files, classifies each block by dialect, and validates it
// deterministically (R10). With --dry-run it prints the per-file
// classification, exemption, and substitution report and exits 0; otherwise
// it validates and exits non-zero listing file:line and reason for every
// failure. Complete blocks of container languages (Rust) compile inside the
// build image of their eval fixture; --require-containers makes a missing
// Docker daemon or image fail the run instead of skipping those blocks, and
// --container-cache-key prints the CI cache key for their package caches.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dash0hq/agent-skills/evals/custom/examples"
	"github.com/dash0hq/agent-skills/evals/custom/examples/otelcolbin"
	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the per-file classification report without validating")
	skillsDir := flag.String("skills-dir", "", "path to the skills/ directory (default: <repo root>/skills)")
	versionsEnv := flag.String("versions-env", "", "path to evals/custom/versions.env (default: <repo root>/evals/custom/versions.env)")
	fixturesDir := flag.String("fixtures-dir", "", "path to evals/custom/fixtures, whose Dockerfiles name the container compile images (default: <repo root>/evals/custom/fixtures)")
	requireContainers := flag.Bool("require-containers", examples.RequireContainersFromEnv(),
		"fail instead of skipping when Docker, a fixture Dockerfile, or a compile image is unavailable (default from "+examples.RequireContainersEnv+")")
	cacheKey := flag.Bool("container-cache-key", false, "print the CI cache key for the container package caches and exit")
	flag.Parse()

	opts := options{
		dryRun:            *dryRun,
		skillsDir:         *skillsDir,
		versionsEnv:       *versionsEnv,
		fixturesDir:       *fixturesDir,
		requireContainers: *requireContainers,
		cacheKey:          *cacheKey,
	}
	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "validate-examples: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	dryRun            bool
	skillsDir         string
	versionsEnv       string
	fixturesDir       string
	requireContainers bool
	cacheKey          bool
}

func run(opts options) error {
	dryRun, skillsDir, versionsEnv, fixturesDir := opts.dryRun, opts.skillsDir, opts.versionsEnv, opts.fixturesDir
	root, err := repoRoot()
	if err != nil && (skillsDir == "" || versionsEnv == "" || fixturesDir == "") {
		return fmt.Errorf("cannot autodetect repo root (%v); pass --skills-dir, --versions-env, and --fixtures-dir", err)
	}
	if skillsDir == "" {
		skillsDir = filepath.Join(root, "skills")
	}
	if versionsEnv == "" {
		versionsEnv = filepath.Join(root, "evals", "custom", "versions.env")
	}
	if fixturesDir == "" {
		fixturesDir = filepath.Join(root, "evals", "custom", "fixtures")
	}
	cacheDir, err := examples.DefaultContainerCacheDir()
	if err != nil {
		return err
	}
	containers := examples.NewContainerRunner(fixturesDir, cacheDir, opts.requireContainers)

	if opts.cacheKey {
		blocks, err := examples.ExtractTree(skillsDir)
		if err != nil {
			return err
		}
		// The Go go.mod, and so the key, carries the OpenTelemetry Go pins.
		versions, err := harness.LoadVersions(versionsEnv)
		if err != nil {
			return err
		}
		containers.Register(examples.NewGoLanguage(versions.OtelGoCoreVersion, versions.OtelGoLogVersion))
		key, err := containers.CacheKey(blocks)
		if err != nil {
			return err
		}
		fmt.Println(key)
		return nil
	}
	if !dryRun && opts.requireContainers && !examples.DockerAvailable() {
		return fmt.Errorf("--require-containers is set but Docker is unavailable")
	}

	var collector *examples.CollectorValidator
	var code *examples.CodeValidator
	if !dryRun {
		versions, err := harness.LoadVersions(versionsEnv)
		if err != nil {
			return err
		}
		binaryPath, err := otelcolbin.Fetch(versions.OtelcolContribVersion, versions.Raw)
		if err != nil {
			return err
		}
		collector = &examples.CollectorValidator{BinaryPath: binaryPath}
		code = examples.NewCodeValidator(versions.OtelGoCoreVersion, versions.OtelGoLogVersion)
		code.Containers = containers
	}

	validator, err := examples.NewValidator(collector, code, dryRun)
	if err != nil {
		return err
	}
	report, err := validator.ValidateTree(skillsDir)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Print(report.Render())
		return nil
	}

	failures := report.Failures()
	fmt.Print(report.Render())
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d validation failure(s):\n", len(failures))
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "  %s:%d: %s\n", failure.File, failure.Line, failure.Detail)
		}
		os.Exit(1)
	}
	fmt.Println("\nAll validated blocks passed.")
	return nil
}

// repoRoot finds the repository root via git, falling back to walking up
// from the working directory until a skills/ directory appears.
func repoRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}
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
			return "", fmt.Errorf("no skills/ directory found walking up from the working directory")
		}
		dir = parent
	}
}
