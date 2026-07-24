package examples

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// tagNormalization maps fence tag aliases to their canonical form. Extend
// this table when new aliases appear in skill content.
var tagNormalization = map[string]string{
	"sh": "bash",
	"js": "javascript",
}

// NormalizeTag returns the canonical fence tag for tag (lowercased, aliases
// resolved).
func NormalizeTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if canonical, ok := tagNormalization[tag]; ok {
		return canonical
	}
	return tag
}

var annotationRe = regexp.MustCompile(`^\s*<!--\s*eval:([a-z0-9-]+)\s*-->\s*$`)

// knownAnnotations are the eval annotations the extractor accepts.
var knownAnnotations = map[Annotation]bool{
	AnnotationSkip:            true,
	AnnotationCollectorConfig: true,
	AnnotationK8s:             true,
	AnnotationFragment:        true,
	AnnotationBad:             true,
}

// ExtractFile extracts every fenced code block from the markdown file at
// path. YAML frontmatter at the top of the file is not a fence. An unknown
// eval annotation is an error.
func ExtractFile(path string) ([]*Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	return extract(path, string(data))
}

func extract(path, content string) ([]*Block, error) {
	lines := strings.Split(content, "\n")
	i := 0
	// Skip YAML frontmatter: an opening --- on line 1 with a closing ---
	// later.
	if len(lines) > 0 && strings.TrimRight(lines[0], " \t") == "---" {
		for j := 1; j < len(lines); j++ {
			if strings.TrimRight(lines[j], " \t") == "---" {
				i = j + 1
				break
			}
		}
	}

	var blocks []*Block
	for ; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "```") {
			continue
		}
		rawTag := strings.TrimSpace(strings.TrimPrefix(line, "```"))
		annotation := AnnotationNone
		if i > 0 {
			if m := annotationRe.FindStringSubmatch(lines[i-1]); m != nil {
				annotation = Annotation(m[1])
				if !knownAnnotations[annotation] {
					return nil, fmt.Errorf("%s:%d: unknown eval annotation %q", path, i, m[1])
				}
			}
		}
		openLine := i + 1
		var body []string
		closed := false
		for i++; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "```") {
				closed = true
				break
			}
			body = append(body, lines[i])
		}
		if !closed {
			return nil, fmt.Errorf("%s:%d: unclosed fence", path, openLine)
		}
		blocks = append(blocks, &Block{
			File:       path,
			Line:       openLine,
			Tag:        NormalizeTag(rawTag),
			RawTag:     rawTag,
			Annotation: annotation,
			Content:    strings.Join(body, "\n"),
		})
	}
	return blocks, nil
}

// ExtractTree extracts blocks from every *.md file under skillsDir, sorted
// by file path.
func ExtractTree(skillsDir string) ([]*Block, error) {
	var files []string
	err := filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("extract: walk %s: %w", skillsDir, err)
	}
	sort.Strings(files)
	var blocks []*Block
	for _, file := range files {
		fileBlocks, err := ExtractFile(file)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, fileBlocks...)
	}
	return blocks, nil
}

// Documents splits a block into YAML documents on standalone --- separator
// lines. Blocks without separators produce one document. The split applies
// only to yaml-tagged and untagged blocks; other tags always produce one
// document.
func (b *Block) Documents() []*Document {
	if b.Tag != "" && b.Tag != "yaml" {
		return []*Document{{Block: b, Index: 0, Line: b.Line + 1, Content: b.Content}}
	}
	lines := strings.Split(b.Content, "\n")
	var docs []*Document
	start := 0
	flush := func(end int) {
		content := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(content) == "" {
			return
		}
		docs = append(docs, &Document{
			Block:   b,
			Index:   len(docs),
			Line:    b.Line + 1 + start,
			Content: content,
		})
	}
	for i, line := range lines {
		if strings.TrimRight(line, " \t") == "---" {
			flush(i)
			start = i + 1
		}
	}
	flush(len(lines))
	if len(docs) == 0 {
		docs = append(docs, &Document{Block: b, Index: 0, Line: b.Line + 1, Content: b.Content})
	}
	return docs
}

var badMarkerRe = regexp.MustCompile(`(?m)(^|\s)(//|#|--|<!--|/\*)\s*BAD\b`)

// HasBadMarker reports whether the block contains a line-comment BAD marker
// (for example "// BAD" or "# BAD").
func (b *Block) HasBadMarker() bool {
	return badMarkerRe.MatchString(b.Content)
}
