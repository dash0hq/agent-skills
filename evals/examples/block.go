// Package examples extracts fenced code blocks from skill markdown files,
// classifies them by dialect, and validates each block deterministically
// (R10). Collector configuration validates via the pinned otelcol-contrib
// binary, OTTL statements parse via pkg/ottl, Kubernetes manifests parse as
// YAML with embedded Collector configuration extracted and validated.
package examples

// Annotation is the value of an HTML-comment annotation placed on the line
// directly above a fence, e.g. <!-- eval:skip -->.
type Annotation string

// Annotation values recognized by the extractor.
const (
	// AnnotationNone means no annotation was present above the fence.
	AnnotationNone Annotation = ""
	// AnnotationSkip exempts the block from validation.
	AnnotationSkip Annotation = "skip"
	// AnnotationCollectorConfig forces classification as Collector config.
	AnnotationCollectorConfig Annotation = "collector-config"
	// AnnotationK8s forces classification as a Kubernetes manifest.
	AnnotationK8s Annotation = "k8s"
	// AnnotationFragment forces classification as a service-less Collector
	// fragment (scaffolded before validation).
	AnnotationFragment Annotation = "fragment"
	// AnnotationBad marks a deliberately wrong example, exempt from
	// validation.
	AnnotationBad Annotation = "bad"
)

// Block is one fenced code block extracted from a markdown file.
type Block struct {
	// File is the path of the markdown file the block came from.
	File string
	// Line is the 1-based line number of the opening fence.
	Line int
	// Tag is the normalized fence language tag ("" for untagged fences).
	Tag string
	// RawTag is the fence language tag as written in the file.
	RawTag string
	// Annotation is the eval annotation above the fence, if any.
	Annotation Annotation
	// Content is the block body without the fence lines.
	Content string
}

// Document is one YAML document inside a Block. Blocks that are not
// multi-document YAML produce exactly one Document spanning the whole
// content.
type Document struct {
	// Block is the block this document belongs to.
	Block *Block
	// Index is the 0-based document index within the block.
	Index int
	// Line is the 1-based file line the document content starts on.
	Line int
	// Content is the document body.
	Content string
}

// Category is the validation category assigned to a document.
type Category string

// Categories assigned by classification.
const (
	// CategoryCollectorConfig is a complete Collector configuration.
	CategoryCollectorConfig Category = "collector-config"
	// CategoryCollectorFragment is a Collector configuration fragment that
	// needs a generated scaffold before validation.
	CategoryCollectorFragment Category = "collector-fragment"
	// CategoryK8sManifest is a Kubernetes manifest.
	CategoryK8sManifest Category = "k8s-manifest"
	// CategoryDockerCompose is a Docker Compose file.
	CategoryDockerCompose Category = "docker-compose"
	// CategoryOTTLStatements is a bare block of OTTL statements or
	// conditions.
	CategoryOTTLStatements Category = "ottl-statements"
	// CategorySkip is a block annotated eval:skip.
	CategorySkip Category = "skip"
	// CategoryBad is a deliberately wrong example (BAD marker or eval:bad).
	CategoryBad Category = "bad"
	// CategoryCodeComplete is a self-contained SDK code block (for example a
	// Go file with a top-level package declaration) routed to the code
	// compiler.
	CategoryCodeComplete Category = "code-complete"
	// CategoryCodeFragment is an SDK code block that is not a self-contained
	// compilation unit (an import snippet, a method body, an elided example,
	// or one annotated <!-- eval:fragment -->). Fragments are reported but
	// not compiled.
	CategoryCodeFragment Category = "code-fragment"
	// CategoryBash is a shell block, exempt by default (scope boundary).
	CategoryBash Category = "bash"
	// CategoryNotValidated is a tagged block with no validator (json, xml,
	// ini, makefile, dockerfile, html, and so on).
	CategoryNotValidated Category = "not-validated"
	// CategoryUnclassified is a yaml or untagged block no heuristic
	// matched; unclassified blocks are validation failures.
	CategoryUnclassified Category = "unclassified"
)

// codeTags are normalized tags treated as SDK code, classified into
// code-complete (self-contained, compiled) and code-fragment (snippet,
// reported but not compiled) by Classify.
var codeTags = map[string]bool{
	"go":         true,
	"python":     true,
	"javascript": true,
	"typescript": true,
	"java":       true,
	"csharp":     true,
	"ruby":       true,
	"php":        true,
	"scala":      true,
}
