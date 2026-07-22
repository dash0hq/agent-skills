// Command validate-examples extracts every fenced code block from the skill
// markdown files, classifies each block by dialect, and validates it
// deterministically (R10). With --dry-run it prints the per-file
// classification, exemption, and substitution report and exits 0; otherwise
// it validates and exits non-zero listing file:line and reason for every
// failure.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dash0hq/agent-skills/evals/examples"
	"github.com/dash0hq/agent-skills/evals/examples/otelcolbin"
	"github.com/dash0hq/agent-skills/evals/harness"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the per-file classification report without validating")
	skillsDir := flag.String("skills-dir", "", "path to the skills/ directory (default: <repo root>/skills)")
	versionsEnv := flag.String("versions-env", "", "path to evals/versions.env (default: <repo root>/evals/versions.env)")
	flag.Parse()

	if err := run(*dryRun, *skillsDir, *versionsEnv); err != nil {
		fmt.Fprintf(os.Stderr, "validate-examples: %v\n", err)
		os.Exit(1)
	}
}

func run(dryRun bool, skillsDir, versionsEnv string) error {
	root, err := repoRoot()
	if err != nil && (skillsDir == "" || versionsEnv == "") {
		return fmt.Errorf("cannot autodetect repo root (%v); pass --skills-dir and --versions-env", err)
	}
	if skillsDir == "" {
		skillsDir = filepath.Join(root, "skills")
	}
	if versionsEnv == "" {
		versionsEnv = filepath.Join(root, "evals", "versions.env")
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
		defer code.Cleanup()
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
