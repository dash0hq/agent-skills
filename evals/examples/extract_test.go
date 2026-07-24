package examples

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	cases := map[string]string{
		"sh":     "bash",
		"js":     "javascript",
		"YAML":   "yaml",
		"go":     "go",
		"":       "",
		" bash ": "bash",
	}
	for input, want := range cases {
		if got := NormalizeTag(input); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExtractFile(t *testing.T) {
	blocks, err := ExtractFile("testdata/extract.md")
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	if len(blocks) != 6 {
		t.Fatalf("got %d blocks, want 6", len(blocks))
	}

	// Frontmatter is not a fence: the first block is the annotated diagram.
	first := blocks[0]
	if first.Annotation != AnnotationSkip {
		t.Errorf("first block annotation = %q, want skip", first.Annotation)
	}
	if first.Tag != "" || first.Content != "ascii diagram" {
		t.Errorf("first block = %+v", first)
	}
	if first.Line != 10 {
		t.Errorf("first block line = %d, want 10", first.Line)
	}
	if strings.Contains(first.Content, "title:") {
		t.Errorf("frontmatter leaked into block content: %q", first.Content)
	}

	// Tag normalization applies during extraction.
	if blocks[1].Tag != "bash" || blocks[1].RawTag != "sh" {
		t.Errorf("second block tag = %q (raw %q), want bash (sh)", blocks[1].Tag, blocks[1].RawTag)
	}
	if blocks[2].Tag != "javascript" || blocks[2].RawTag != "js" {
		t.Errorf("third block tag = %q (raw %q), want javascript (js)", blocks[2].Tag, blocks[2].RawTag)
	}

	// The go block carries a BAD marker.
	if !blocks[4].HasBadMarker() {
		t.Errorf("go block should carry a BAD marker")
	}
	if blocks[5].HasBadMarker() {
		t.Errorf("prose block should not carry a BAD marker")
	}
}

func TestDocumentsMultiDocSplit(t *testing.T) {
	blocks, err := ExtractFile("testdata/extract.md")
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	yamlBlock := blocks[3]
	if yamlBlock.Tag != "yaml" {
		t.Fatalf("expected yaml block, got %q", yamlBlock.Tag)
	}
	docs := yamlBlock.Documents()
	if len(docs) != 2 {
		t.Fatalf("got %d documents, want 2", len(docs))
	}
	if !strings.Contains(docs[0].Content, "kind: ConfigMap") {
		t.Errorf("first document content = %q", docs[0].Content)
	}
	if !strings.Contains(docs[1].Content, "kind: Secret") {
		t.Errorf("second document content = %q", docs[1].Content)
	}
	if docs[1].Line <= docs[0].Line {
		t.Errorf("document lines not increasing: %d then %d", docs[0].Line, docs[1].Line)
	}
	// The second document starts on the line after the --- separator.
	if want := yamlBlock.Line + 6; docs[1].Line != want {
		t.Errorf("second document line = %d, want %d", docs[1].Line, want)
	}
}

func TestDocumentsSingleForNonYAML(t *testing.T) {
	block := &Block{Tag: "go", Line: 5, Content: "a\n---\nb"}
	if docs := block.Documents(); len(docs) != 1 {
		t.Fatalf("go block split into %d documents, want 1", len(docs))
	}
}

func TestExtractUnknownAnnotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	content := "# T\n\n<!-- eval:bogus -->\n```yaml\na: b\n```\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractFile(path); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("expected unknown-annotation error, got %v", err)
	}
}

func TestExtractUnclosedFence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "open.md")
	if err := os.WriteFile(path, []byte("```yaml\na: b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractFile(path); err == nil || !strings.Contains(err.Error(), "unclosed") {
		t.Fatalf("expected unclosed-fence error, got %v", err)
	}
}
