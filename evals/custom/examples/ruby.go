package examples

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// rubyPins is the pinned gem set the Ruby examples are checked against, and
// the static checker that runs in the container. Regenerate Gemfile.lock after
// editing Gemfile (see the Ruby section of evals/custom/README.md).
//
//go:embed containers/ruby/Gemfile containers/ruby/Gemfile.lock containers/ruby/check.rb
var rubyPins embed.FS

// rubyCheckDir is the project directory holding the checker and the list of
// files it checks; example blocks may not write into it.
const rubyCheckDir = ".eval"

// rubyLanguage statically checks the complete Ruby blocks of one rule file as
// one project, inside the ruby-service fixture's image, without executing
// example code.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "# <path>.rb" comment is
//     written to that path; any other complete block (one with a top-level
//     require of a library) to examples/block_<line>.rb.
//  2. The project uses the pinned Gemfile and Gemfile.lock under
//     containers/ruby/; examples require nothing outside them and the Ruby
//     standard library.
//  3. Every file is checked by containers/ruby/check.rb: "ruby -wc" (syntax),
//     each top-level literal require must load, every constant path rooted
//     at a constant a required library defines must resolve, and every method
//     called on such a constant must exist and accept the call's arguments.
//  4. Names rooted outside the required libraries (application state such as
//     order, logger, or Rails) are not checked.
type rubyLanguage struct{}

func (rubyLanguage) Name() string    { return "ruby" }
func (rubyLanguage) Tags() []string  { return []string{"ruby"} }
func (rubyLanguage) Fixture() string { return "ruby-service" }

func (rubyLanguage) ManifestFiles() []string { return []string{"Gemfile", "Gemfile.lock"} }

// Caches mounts the .gem download cache, which CI caches, at Bundler's
// default cache path, and BUNDLE_PATH, which keeps the per-file installs
// incremental.
func (rubyLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/work/vendor/cache"},
		{Name: "bundle", Path: "/cache/bundle"},
	}
}

// Env pins Bundler to the lockfile (BUNDLE_FROZEN) and to the image's own
// Bundler (BUNDLE_VERSION=system), so it never downloads another Bundler.
func (rubyLanguage) Env() []string {
	return []string{"BUNDLE_PATH=/cache/bundle", "BUNDLE_FROZEN=true", "BUNDLE_VERSION=system"}
}

// rubyFetchScript downloads the .gem file of every locked gem for the
// "ruby" platform or the container's platform into vendor/cache, without
// installing anything: native extensions build only in the offline phase.
const rubyFetchScript = `require "bundler"
require "fileutils"
require "rubygems/remote_fetcher"
cache = "vendor/cache"
FileUtils.mkdir_p(cache)
fetcher = Gem::RemoteFetcher.fetcher
Bundler::LockfileParser.new(Bundler.read_file("Gemfile.lock")).specs.each do |spec|
  next unless spec.platform.to_s == "ruby" || Gem::Platform.local =~ spec.platform
  target = File.join(cache, "#{spec.full_name}.gem")
  next if File.exist?(target)
  remote = spec.source.remotes.first.to_s.chomp("/")
  File.binwrite("#{target}.tmp", fetcher.fetch_path("#{remote}/gems/#{spec.full_name}.gem"))
  File.rename("#{target}.tmp", target)
end`

func (rubyLanguage) FetchCommand() []string {
	return []string{"ruby", "-e", rubyFetchScript}
}

func (rubyLanguage) CheckLabel() string { return "ruby check" }

// CheckCommand installs the bundle from the downloaded .gem files, building
// native extensions offline, then runs the static checker.
func (rubyLanguage) CheckCommand() []string {
	return []string{"sh", "-c", "bundle install --local --quiet && bundle exec ruby " +
		rubyCheckDir + "/check.rb " + rubyCheckDir + "/files.txt"}
}

// rubyDiagnosticRe matches a check.rb error line, for example
// "examples/block_5.rb:3:7: error: uninitialized constant Foo::Bar".
var rubyDiagnosticRe = regexp.MustCompile(`^(\S+\.rb):(\d+):(\d+): error: (.*)$`)

func (rubyLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		m := rubyDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		diagnostics = append(diagnostics, Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: m[4]})
	}
	return diagnostics
}

// rubyPathCommentRe matches the "# <path>.rb" first line naming a block's
// file.
var rubyPathCommentRe = regexp.MustCompile(`^#\s*([A-Za-z0-9_./-]+\.rb)\s*$`)

// rubyRequireRe matches a top-level require of a library at column 0, for
// example `require 'opentelemetry/sdk'`; require_relative does not match.
var rubyRequireRe = regexp.MustCompile(`(?m)^require[\s(]+['"][^'"]+['"]`)

// rubyBlockPath returns the path named on the block's first non-empty line,
// or "".
func rubyBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := rubyPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// isCompleteRuby reports whether a Ruby block is a self-contained file: its
// first non-empty line names the file with a "# <path>.rb" comment, or it
// requires a library at the top level.
func isCompleteRuby(content string) bool {
	return rubyBlockPath(content) != "" || rubyRequireRe.MatchString(content)
}

func (rubyLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := rubyBlockPath(block.Content)
		if p == "" {
			p = fmt.Sprintf("examples/block_%d.rb", block.Line)
		}
		if path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "..") ||
			strings.HasPrefix(p, "vendor/") || strings.HasPrefix(p, rubyCheckDir+"/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string]string{}
	var list []string
	for p, block := range owner {
		files[p] = block.Content + "\n"
		list = append(list, p)
	}
	sort.Strings(list)
	files[rubyCheckDir+"/files.txt"] = strings.Join(list, "\n") + "\n"
	for _, name := range []string{"Gemfile", "Gemfile.lock", "check.rb"} {
		data, err := rubyPins.ReadFile("containers/ruby/" + name)
		if err != nil {
			return nil, err
		}
		target := name
		if name == "check.rb" {
			target = rubyCheckDir + "/check.rb"
		}
		files[target] = string(data)
	}

	for p, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
