package examples

import (
	"strings"
	"testing"
)

func TestLintDockerfileENVForms(t *testing.T) {
	cases := []struct {
		name    string
		content string
		// wantMessage is a substring of the single expected issue message;
		// empty means no issues expected.
		wantMessage string
		wantLine    int
		wantBreaks  bool
	}{
		{
			name:    "quoted name=value form passes",
			content: "FROM node:22-alpine\nENV NODE_OPTIONS=\"--require @opentelemetry/auto-instrumentations-node/register\"\n",
		},
		{
			name:    "unquoted name=value without spaces passes",
			content: "FROM node:22-alpine\nENV NODE_ENV=production\n",
		},
		{
			name:    "multiple name=value pairs pass",
			content: "FROM node:22-alpine\nENV NODE_ENV=production OTEL_TRACES_EXPORTER=otlp\n",
		},
		{
			name:    "name=value pair with quoted spaces passes",
			content: "FROM node:22-alpine\nENV A=\"b c\" D=e\n",
		},
		{
			name: "issue 120 regression: legacy form with multi-token value breaks classic builders",
			content: "FROM node:22-alpine\n" +
				"ENV NODE_OPTIONS --require @opentelemetry/auto-instrumentations-node/register\n",
			wantMessage: "classic",
			wantLine:    2,
			wantBreaks:  true,
		},
		{
			name:        "legacy two-token form is flagged but does not break builds",
			content:     "FROM node:22-alpine\nENV NODE_ENV production\n",
			wantMessage: "legacy",
			wantLine:    2,
			wantBreaks:  false,
		},
		{
			name:        "mixed pair and bare token breaks",
			content:     "FROM node:22-alpine\nENV A=1 B\n",
			wantMessage: "name=value",
			wantLine:    2,
			wantBreaks:  true,
		},
		{
			name: "legacy form split over a continuation breaks",
			content: "FROM node:22-alpine\n" +
				"ENV NODE_OPTIONS \\\n    --require @opentelemetry/auto-instrumentations-node/register\n",
			wantMessage: "classic",
			wantLine:    2,
			wantBreaks:  true,
		},
		{
			name:        "legacy multi-token LABEL breaks too",
			content:     "FROM scratch\nLABEL description an example image\n",
			wantMessage: "classic",
			wantLine:    2,
			wantBreaks:  true,
		},
		{
			name:    "comment lines and other instructions are ignored",
			content: "# ENV NODE_OPTIONS --require pkg\nFROM node:22-alpine\nRUN echo ENV NODE_OPTIONS --require pkg\nCMD [\"node\", \"server.js\"]\n",
		},
		{
			name:    "heredoc bodies are not parsed as instructions",
			content: "FROM node:22-alpine\nRUN <<EOF\nENV=nonsense but inside a shell script\necho hi\nEOF\nENV NODE_ENV=production\n",
		},
		{
			name:    "lowercase env instruction with valid pair passes",
			content: "FROM node:22-alpine\nenv NODE_ENV=production\n",
		},
		{
			name:        "lowercase env instruction with legacy multi-token form breaks",
			content:     "FROM node:22-alpine\nenv NODE_OPTIONS --require pkg/register\n",
			wantMessage: "classic",
			wantLine:    2,
			wantBreaks:  true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			issues := LintDockerfile(testCase.content)
			if testCase.wantMessage == "" {
				if len(issues) != 0 {
					t.Fatalf("expected no issues, got %v", issues)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
			}
			issue := issues[0]
			if !strings.Contains(issue.Message, testCase.wantMessage) {
				t.Errorf("message %q does not contain %q", issue.Message, testCase.wantMessage)
			}
			if issue.Line != testCase.wantLine {
				t.Errorf("line: got %d, want %d", issue.Line, testCase.wantLine)
			}
			if issue.BreaksClassicBuilder != testCase.wantBreaks {
				t.Errorf("BreaksClassicBuilder: got %t, want %t", issue.BreaksClassicBuilder, testCase.wantBreaks)
			}
		})
	}
}
