package examples

import (
	"fmt"
	"regexp"
	"strings"
)

// The Dockerfile linter guards the ENV/LABEL instruction forms in dockerfile
// blocks (and, via the scenario build gate, in agent-written Dockerfiles).
// It exists because of a real regression (issue #120): skill-following agents
// wrote the legacy space-separated form
//
//	ENV NODE_OPTIONS --require @opentelemetry/auto-instrumentations-node/register
//
// which BuildKit accepts but the classic builder — still what `docker build`
// via the Engine API uses on stock GitHub Actions runners — rejects at parse
// time ("Syntax error - can't find = in ... Must be of the form: name=value")
// whenever the value contains spaces. The linter is deliberately narrow: it
// checks only the name=value contract of ENV and LABEL, the one Dockerfile
// parse rule that differs between builders in a way that broke real builds.

// DockerfileIssue is one problem found in a Dockerfile.
type DockerfileIssue struct {
	// Line is the 1-based line the instruction starts on.
	Line int
	// Message describes the problem.
	Message string
	// BreaksClassicBuilder is true when the instruction fails a real build
	// on at least one supported builder (the classic builder's name=value
	// parse rule); false for the deprecated-but-buildable two-token legacy
	// form.
	BreaksClassicBuilder bool
}

func (i DockerfileIssue) String() string {
	return fmt.Sprintf("line %d: %s", i.Line, i.Message)
}

// heredocRe matches a heredoc opener token in an instruction, e.g. <<EOF,
// <<-EOF, or <<"EOF", capturing the delimiter.
var heredocRe = regexp.MustCompile("<<-?['\"]?([A-Za-z_][A-Za-z0-9_]*)['\"]?")

// LintDockerfile checks every ENV and LABEL instruction in content for the
// name=value contract and returns one issue per offending instruction.
func LintDockerfile(content string) []DockerfileIssue {
	var issues []DockerfileIssue
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Assemble the logical instruction across backslash continuations,
		// skipping interleaved comment lines like the Dockerfile parser does.
		startLine := i + 1
		logical := lines[i]
		for strings.HasSuffix(strings.TrimRight(logical, " \t"), "\\") && i+1 < len(lines) {
			logical = strings.TrimSuffix(strings.TrimRight(logical, " \t"), "\\")
			i++
			next := lines[i]
			if strings.HasPrefix(strings.TrimSpace(next), "#") {
				continue
			}
			logical += " " + next
		}
		// Skip heredoc bodies (RUN <<EOF ... EOF): their lines are shell, not
		// Dockerfile instructions.
		if m := heredocRe.FindStringSubmatch(logical); m != nil {
			delimiter := m[1]
			for i+1 < len(lines) {
				i++
				if strings.TrimSpace(lines[i]) == delimiter {
					break
				}
			}
		}
		instruction, rest, _ := strings.Cut(strings.TrimSpace(logical), " ")
		keyword := strings.ToUpper(instruction)
		if keyword != "ENV" && keyword != "LABEL" {
			continue
		}
		if issue := lintNameVal(keyword, strings.TrimSpace(rest)); issue != nil {
			issue.Line = startLine
			issues = append(issues, *issue)
		}
	}
	return issues
}

// lintNameVal applies the name=value contract to the arguments of one ENV or
// LABEL instruction. It returns nil when the instruction is well-formed.
func lintNameVal(keyword, rest string) *DockerfileIssue {
	words := splitDockerWords(rest)
	if len(words) == 0 {
		return &DockerfileIssue{
			Message:              keyword + " has no arguments",
			BreaksClassicBuilder: true,
		}
	}
	if strings.Contains(words[0], "=") {
		// name=value form: every word must be a pair.
		for _, word := range words {
			if !strings.Contains(word, "=") {
				return &DockerfileIssue{
					Message: fmt.Sprintf(
						"%s mixes name=value pairs with the bare token %q — every argument must be of the form name=value",
						keyword, word),
					BreaksClassicBuilder: true,
				}
			}
		}
		return nil
	}
	// Legacy space-separated form.
	switch len(words) {
	case 1:
		return &DockerfileIssue{
			Message:              fmt.Sprintf("%s %s has a name but no value", keyword, words[0]),
			BreaksClassicBuilder: true,
		}
	case 2:
		return &DockerfileIssue{
			Message: fmt.Sprintf(
				"%s %s uses the legacy space-separated form; use %s %s=%q instead",
				keyword, words[0], keyword, words[0], words[1]),
			BreaksClassicBuilder: false,
		}
	default:
		value := strings.TrimSpace(strings.TrimPrefix(rest, words[0]))
		return &DockerfileIssue{
			Message: fmt.Sprintf(
				"%s %s uses the legacy space-separated form with a value containing spaces; "+
					"the classic Docker builder rejects it at parse time — use %s %s=%q instead",
				keyword, words[0], keyword, words[0], value),
			BreaksClassicBuilder: true,
		}
	}
}

// splitDockerWords splits instruction arguments on unquoted whitespace,
// keeping quotes and backslash escapes inside the returned words, mirroring
// the Dockerfile parser's word splitting closely enough for the name=value
// check.
func splitDockerWords(s string) []string {
	var words []string
	var current strings.Builder
	inSingle, inDouble, escaped := false, false, false
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			current.WriteRune(r)
			escaped = true
		case r == '\'' && !inDouble:
			current.WriteRune(r)
			inSingle = !inSingle
		case r == '"' && !inSingle:
			current.WriteRune(r)
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return words
}
