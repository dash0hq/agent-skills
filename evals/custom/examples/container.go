package examples

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContainerLanguage is the plug-in contract for compiling one language's
// complete code blocks inside the build image of that language's eval fixture.
// The runner calls it once per rule file: Assemble lays out a project from the
// file's blocks, then FetchCommand runs with network access and CheckCommand
// runs with --network none, so third-party build scripts and macros never see
// the network.
type ContainerLanguage interface {
	// Name identifies the language in reports and names its host cache
	// directory, for example "rust".
	Name() string
	// Tags are the normalized fence tags the language compiles; blocks of
	// all of them in one rule file share one project.
	Tags() []string
	// Fixture is the fixture directory under evals/custom/fixtures whose
	// Dockerfile names the compile image (see BuildImage).
	Fixture() string
	// Assemble writes a project into dir from the complete code blocks of one
	// rule file and every block of that file (for manifest blocks such as
	// toml). It returns the project-relative path each code block was written
	// to.
	Assemble(dir string, code []*Block, file []*Block) (map[*Block]string, error)
	// ManifestFiles lists the project-relative manifest files Assemble
	// generates; their content keys the CI package cache.
	ManifestFiles() []string
	// Caches lists the host cache directories mounted into the container.
	Caches() []ContainerCache
	// Env lists KEY=VALUE environment variables set in the container.
	Env() []string
	// FetchCommand downloads dependencies; it runs with network access.
	FetchCommand() []string
	// CheckCommand compiles the project; it runs with --network none.
	CheckCommand() []string
	// CheckLabel names the check in failure details, for example
	// "cargo check".
	CheckLabel() string
	// Diagnostics extracts the error diagnostics from the check output.
	Diagnostics(output string) []Diagnostic
}

// ContainerCache is one host cache directory mounted into the compile
// container.
type ContainerCache struct {
	// Name is the subdirectory under <cache root>/<tag>/ on the host.
	Name string
	// Path is the mount point inside the container.
	Path string
}

// Diagnostic is one compiler error located in a project file.
type Diagnostic struct {
	// Path is the project-relative file path, for example "src/main.rs".
	Path string
	// Line and Column are 1-based positions in that file.
	Line, Column int
	// Message is the compiler message without the location prefix.
	Message string
}

// ContainerCacheEnv overrides the container cache root
// (default: <user cache dir>/agent-skills-evals-containers).
const ContainerCacheEnv = "EVAL_CONTAINER_CACHE_DIR"

// RequireContainersEnv makes container compiles fail closed when set to "1"
// or "true": a missing Docker daemon, fixture Dockerfile, or image pull
// fails the block instead of reporting it skipped-no-toolchain.
const RequireContainersEnv = "EVAL_REQUIRE_CONTAINERS"

// RequireContainersFromEnv reports whether RequireContainersEnv is set.
func RequireContainersFromEnv() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(RequireContainersEnv)))
	return value == "1" || value == "true"
}

// DefaultContainerCacheDir returns the container cache root: the
// EVAL_CONTAINER_CACHE_DIR override, or agent-skills-evals-containers under
// the user cache directory. It lives under the home directory on macOS, which
// Docker Desktop and Colima share with their VM by default.
func DefaultContainerCacheDir() (string, error) {
	if dir := os.Getenv(ContainerCacheEnv); dir != "" {
		return filepath.Abs(dir)
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("containers: resolve cache dir: %w", err)
	}
	return filepath.Join(root, "agent-skills-evals-containers"), nil
}

// ContainerLanguages returns every registered container language plug-in.
func ContainerLanguages() []ContainerLanguage {
	return []ContainerLanguage{rustLanguage{}, javascriptLanguage{}, pythonLanguage{}}
}

// Timeouts for the container phases.
const (
	pullTimeout  = 15 * time.Minute
	fetchTimeout = 15 * time.Minute
	checkTimeout = 30 * time.Minute
)

// ContainerRunner compiles the complete code blocks of container languages,
// one container per rule file and language. Blocks are queued by Enqueue
// while the tree is walked and compiled by Run once every block is known. The
// zero value is not usable; build one with NewContainerRunner.
type ContainerRunner struct {
	// FixturesDir is the evals/custom/fixtures directory.
	FixturesDir string
	// CacheDir is the host cache root.
	CacheDir string
	// Require fails blocks instead of skipping them when Docker, the fixture
	// Dockerfile, or the image is unavailable.
	Require bool

	languages map[string]ContainerLanguage
	queue     []queuedBlock

	// available reports whether Docker is usable; replaced in tests.
	available     func() bool
	availableOnce sync.Once
	availableOK   bool
}

type queuedBlock struct {
	result *Result
	block  *Block
}

// NewContainerRunner builds a runner for every registered container
// language.
func NewContainerRunner(fixturesDir, cacheDir string, require bool) *ContainerRunner {
	languages := map[string]ContainerLanguage{}
	for _, language := range ContainerLanguages() {
		for _, tag := range language.Tags() {
			languages[tag] = language
		}
	}
	return &ContainerRunner{
		FixturesDir: fixturesDir,
		CacheDir:    cacheDir,
		Require:     require,
		languages:   languages,
		available:   DockerAvailable,
	}
}

// Handles reports whether tag has a container language plug-in.
func (r *ContainerRunner) Handles(tag string) bool {
	_, ok := r.languages[tag]
	return ok
}

// Enqueue defers the compile of a complete block until Run.
func (r *ContainerRunner) Enqueue(result *Result, block *Block) {
	r.queue = append(r.queue, queuedBlock{result: result, block: block})
}

// Run compiles every queued block, one container per rule file and language,
// and records the outcome on each queued result. all is every block of the
// tree; the language reads manifest blocks of the same file from it.
func (r *ContainerRunner) Run(all []*Block) {
	for _, group := range r.groups(r.queue) {
		r.runGroup(group, blocksOfFile(all, group.file))
	}
	r.queue = nil
}

type containerGroup struct {
	file     string
	language ContainerLanguage
	queued   []queuedBlock
}

// groups splits queued blocks by rule file and language, in queue order.
func (r *ContainerRunner) groups(queue []queuedBlock) []*containerGroup {
	var groups []*containerGroup
	index := map[string]*containerGroup{}
	for _, q := range queue {
		language := r.languages[q.block.Tag]
		key := q.block.File + "\x00" + language.Name()
		group, ok := index[key]
		if !ok {
			group = &containerGroup{file: q.block.File, language: language}
			index[key] = group
			groups = append(groups, group)
		}
		group.queued = append(group.queued, q)
	}
	return groups
}

func blocksOfFile(all []*Block, file string) []*Block {
	var blocks []*Block
	for _, block := range all {
		if block.File == file {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (g *containerGroup) codeBlocks() []*Block {
	blocks := make([]*Block, len(g.queued))
	for i, q := range g.queued {
		blocks[i] = q.block
	}
	return blocks
}

func (g *containerGroup) setAll(status Status, detail string) {
	for _, q := range g.queued {
		q.result.Status = status
		q.result.Detail = detail
	}
}

// unavailable records an environment problem: skipped-no-toolchain locally,
// failed when the runner is required to fail closed.
func (r *ContainerRunner) unavailable(group *containerGroup, reason string) {
	tag := group.language.Name()
	if r.Require {
		group.setAll(StatusFailed, fmt.Sprintf("%s; complete %s block not compiled (%s set, failing closed)", reason, tag, RequireContainersEnv))
		return
	}
	group.setAll(StatusSkippedNoToolchain, fmt.Sprintf("%s; complete %s block not compiled", reason, tag))
}

func (r *ContainerRunner) runGroup(group *containerGroup, fileBlocks []*Block) {
	language := group.language
	r.availableOnce.Do(func() { r.availableOK = r.available() })
	if !r.availableOK {
		r.unavailable(group, "docker unavailable")
		return
	}
	image, err := r.Image(language)
	if err != nil {
		r.unavailable(group, err.Error())
		return
	}
	if err := ensureImage(image); err != nil {
		r.unavailable(group, err.Error())
		return
	}

	langDir := filepath.Join(r.CacheDir, language.Name())
	if err := os.MkdirAll(filepath.Join(langDir, "work"), 0o755); err != nil {
		group.setAll(StatusFailed, "container work dir: "+err.Error())
		return
	}
	// The work dir sits under the cache root rather than the system temp
	// dir so it is inside the home directory that macOS Docker VMs share.
	workDir, err := os.MkdirTemp(filepath.Join(langDir, "work"), "project-")
	if err != nil {
		group.setAll(StatusFailed, "container work dir: "+err.Error())
		return
	}
	defer os.RemoveAll(workDir)

	paths, err := language.Assemble(workDir, group.codeBlocks(), fileBlocks)
	if err != nil {
		group.setAll(StatusFailed, fmt.Sprintf("assemble %s project: %v", language.Name(), err))
		return
	}

	args, err := r.runArgs(language, langDir, workDir)
	if err != nil {
		group.setAll(StatusFailed, err.Error())
		return
	}
	fetch := language.FetchCommand()
	fetchArgs := slices.Concat(args, []string{image}, fetch)
	if out, err := runDocker(fetchTimeout, fetchArgs...); err != nil {
		group.setAll(StatusFailed, fmt.Sprintf("%s failed (network phase): %s", strings.Join(fetch, " "), trimOutput(out, err)))
		return
	}
	check := language.CheckCommand()
	checkArgs := slices.Concat(args, []string{"--network", "none", image}, check)
	out, err := runDocker(checkTimeout, checkArgs...)
	if err != nil {
		r.attribute(group, paths, language.CheckLabel(), out, err)
		return
	}
	group.setAll(StatusValidated, "")
}

// runArgs builds the shared docker run arguments: the project and cache
// mounts, the language environment, and the host user so files written to
// the mounts stay owned by (and removable by) this process.
func (r *ContainerRunner) runArgs(language ContainerLanguage, langDir, workDir string) ([]string, error) {
	args := []string{"run", "--rm",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "HOME=/tmp",
		"-v", workDir + ":/work", "-w", "/work",
	}
	for _, cache := range language.Caches() {
		hostDir := filepath.Join(langDir, cache.Name)
		if err := os.MkdirAll(hostDir, 0o755); err != nil {
			return nil, fmt.Errorf("container cache dir: %w", err)
		}
		args = append(args, "-v", hostDir+":"+cache.Path)
	}
	for _, env := range language.Env() {
		args = append(args, "-e", env)
	}
	return args, nil
}

// attribute records a failed check: blocks named by a diagnostic get those
// diagnostics, rewritten to rule-file lines; the other blocks of the project
// fail with a pointer to them. When no diagnostic names a block, every block
// gets the raw output.
func (r *ContainerRunner) attribute(group *containerGroup, paths map[*Block]string, command string, out []byte, err error) {
	byPath := map[string]*Block{}
	for block, path := range paths {
		byPath[path] = block
	}
	messages := map[*Block][]string{}
	for _, diagnostic := range group.language.Diagnostics(string(out)) {
		block, ok := byPath[diagnostic.Path]
		if !ok {
			continue
		}
		// Content line n of a block sits n lines below its opening fence.
		messages[block] = append(messages[block], fmt.Sprintf("%s:%d:%d: %s",
			block.File, block.Line+diagnostic.Line, diagnostic.Column, diagnostic.Message))
	}
	if len(messages) == 0 {
		group.setAll(StatusFailed, fmt.Sprintf("%s failed: %s", command, trimOutput(out, err)))
		return
	}
	var failing []string
	for _, q := range group.queued {
		if _, ok := messages[q.block]; ok {
			failing = append(failing, fmt.Sprintf("%s (line %d)", paths[q.block], q.result.Line))
		}
	}
	for _, q := range group.queued {
		q.result.Status = StatusFailed
		if lines, ok := messages[q.block]; ok {
			q.result.Detail = fmt.Sprintf("%s failed: %s", command, strings.Join(lines, "; "))
		} else {
			q.result.Detail = fmt.Sprintf("%s failed in the project assembled from this file; errors in %s", command, strings.Join(failing, ", "))
		}
	}
}

// trimOutput returns the last lines of a failed command's output, or the
// error when the command printed nothing.
func trimOutput(out []byte, err error) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return err.Error()
	}
	const maxLines = 40
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = append([]string{"..."}, lines[len(lines)-maxLines:]...)
	}
	return strings.Join(lines, "\n")
}

// Image returns the compile image for language, read from the fixture
// Dockerfile.
func (r *ContainerRunner) Image(language ContainerLanguage) (string, error) {
	return BuildImage(filepath.Join(r.FixturesDir, language.Fixture(), "Dockerfile"))
}

// fromRe matches a FROM line, capturing the image and the optional stage
// name; keywords are case-insensitive as in Dockerfiles.
var fromRe = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)(?:\s+AS\s+(\S+))?\s*$`)

// BuildImage returns the compile image named by the Dockerfile at path, so
// snippet compiles use the toolchain the fixture builds with: the image of the
// "FROM <image> AS build" stage, or, in a single-stage Dockerfile, the image
// of its only FROM line.
func BuildImage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("fixture Dockerfile %s not found", path)
		}
		return "", fmt.Errorf("fixture Dockerfile: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var images []string
	for scanner.Scan() {
		m := fromRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		if strings.EqualFold(m[2], "build") {
			return m[1], nil
		}
		images = append(images, m[1])
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("fixture Dockerfile %s: %w", path, err)
	}
	if len(images) == 1 {
		return images[0], nil
	}
	return "", fmt.Errorf("fixture Dockerfile %s has neither a \"FROM <image> AS build\" line nor a single FROM line", path)
}

// CacheKey returns the CI cache key for the container package caches: the
// compile image of every language with complete blocks in the tree plus a
// hash of the manifests generated for them. It needs no Docker. A tree
// without container-language blocks yields "containers-none".
func (r *ContainerRunner) CacheKey(all []*Block) (string, error) {
	var queue []queuedBlock
	for _, block := range all {
		for _, doc := range block.Documents() {
			if Classify(doc) == CategoryCodeComplete && r.Handles(block.Tag) {
				queue = append(queue, queuedBlock{result: &Result{}, block: block})
			}
		}
	}
	groups := r.groups(queue)
	if len(groups) == 0 {
		return "containers-none", nil
	}
	images := map[string]string{}
	hash := sha256.New()
	tmp, err := os.MkdirTemp("", "eval-container-key-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for i, group := range groups {
		language := group.language
		image, err := r.Image(language)
		if err != nil {
			return "", err
		}
		images[language.Name()] = image
		dir := filepath.Join(tmp, fmt.Sprint(i))
		if _, err := language.Assemble(dir, group.codeBlocks(), blocksOfFile(all, group.file)); err != nil {
			// The validation run reports the assembly error on the blocks;
			// the key only needs to be deterministic.
			fmt.Fprintf(hash, "%s: %v\n", group.file, err)
			continue
		}
		for _, manifest := range language.ManifestFiles() {
			data, err := os.ReadFile(filepath.Join(dir, manifest))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(hash, "%s %s\n", group.file, manifest)
			hash.Write(data)
		}
	}
	tags := make([]string, 0, len(images))
	for tag := range images {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	var parts []string
	for _, tag := range tags {
		parts = append(parts, tag+"-"+sanitizeKey(images[tag]))
	}
	return "containers-" + strings.Join(parts, "-") + "-" + hex.EncodeToString(hash.Sum(nil))[:16], nil
}

var keyUnsafeRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeKey(s string) string {
	return keyUnsafeRe.ReplaceAllString(s, "_")
}

// DockerAvailable reports whether the docker CLI and a reachable daemon are
// available.
func DockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

// ensureImage pulls image unless it is already present locally.
func ensureImage(image string) error {
	if _, err := runDocker(time.Minute, "image", "inspect", image); err == nil {
		return nil
	}
	if out, err := runDocker(pullTimeout, "pull", image); err != nil {
		return fmt.Errorf("docker pull %s failed: %s", image, trimOutput(out, err))
	}
	return nil
}

// runDocker runs the docker CLI with a timeout and returns its combined
// output.
func runDocker(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("docker %s timed out after %s", args[0], timeout)
	}
	return out, err
}
