package examples

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Status is the outcome recorded for one document.
type Status string

// Statuses recorded in the report.
const (
	// StatusValidated means the document passed its validator.
	StatusValidated Status = "validated"
	// StatusExempt means the document is exempt from validation (skip,
	// bad, bash, code-fragment, not-validated).
	StatusExempt Status = "exempt"
	// StatusFailed means validation failed (including unclassified
	// documents).
	StatusFailed Status = "failed"
	// StatusClassified is used in dry-run mode: classification only, no
	// validation.
	StatusClassified Status = "classified"
	// StatusSkippedNoToolchain means a code-complete block could not be
	// compiled because its toolchain is unavailable here (a non-Go language
	// whose fixture-image compiler is a follow-up, or a missing go binary).
	// It is visible in the report and never counts as validated.
	StatusSkippedNoToolchain Status = "skipped-no-toolchain"
)

// Result is the outcome for one document of one block.
type Result struct {
	// File is the markdown file path.
	File string
	// Line is the 1-based line of the document content (fence line + 1
	// for single-document blocks).
	Line int
	// Tag is the normalized fence tag.
	Tag string
	// Category is the classification.
	Category Category
	// Annotation is the eval annotation on the block, if any.
	Annotation Annotation
	// Status is the outcome.
	Status Status
	// Detail carries the failure reason or exemption note.
	Detail string
	// Substitutions lists placeholder substitutions applied before
	// validation.
	Substitutions []string
}

// Report is the outcome of a validation run.
type Report struct {
	// Results holds one entry per document, in file order.
	Results []*Result
}

// Failures returns the failed results.
func (r *Report) Failures() []*Result {
	var failures []*Result
	for _, result := range r.Results {
		if result.Status == StatusFailed {
			failures = append(failures, result)
		}
	}
	return failures
}

// Validator runs the full example-validation pipeline.
type Validator struct {
	// OTTL parses OTTL statements.
	OTTL *OTTLValidator
	// Collector validates Collector configurations; nil in dry-run mode.
	Collector *CollectorValidator
	// Code compiles complete SDK code blocks; nil in dry-run mode.
	Code *CodeValidator
	// DryRun classifies and reports without validating.
	DryRun bool
}

// NewValidator builds a validator. collector and code may be nil only when
// dryRun is true.
func NewValidator(collector *CollectorValidator, code *CodeValidator, dryRun bool) (*Validator, error) {
	ottlValidator, err := NewOTTLValidator()
	if err != nil {
		return nil, err
	}
	if !dryRun && (collector == nil || code == nil) {
		return nil, fmt.Errorf("validator: collector and code validators required outside dry-run mode")
	}
	return &Validator{OTTL: ottlValidator, Collector: collector, Code: code, DryRun: dryRun}, nil
}

// ValidateTree extracts, classifies, and validates every fenced block under
// skillsDir.
func (v *Validator) ValidateTree(skillsDir string) (*Report, error) {
	blocks, err := ExtractTree(skillsDir)
	if err != nil {
		return nil, err
	}
	report := &Report{}
	for _, block := range blocks {
		for _, doc := range block.Documents() {
			report.Results = append(report.Results, v.validateDocument(doc)...)
		}
	}
	return report, nil
}

func (v *Validator) validateDocument(doc *Document) []*Result {
	block := doc.Block
	category := Classify(doc)
	result := &Result{
		File:       block.File,
		Line:       doc.Line,
		Tag:        block.Tag,
		Category:   category,
		Annotation: block.Annotation,
	}

	if v.DryRun {
		result.Status = StatusClassified
		switch category {
		case CategoryCollectorConfig, CategoryCollectorFragment, CategoryK8sManifest:
			_, result.Substitutions = SubstitutePlaceholders(doc.Content)
		case CategoryUnclassified:
			result.Status = StatusFailed
			result.Detail = "unclassified block: tag it, or annotate with <!-- eval:... -->"
		}
		return []*Result{result}
	}

	switch category {
	case CategorySkip:
		result.Status = StatusExempt
		result.Detail = "annotated eval:skip"
	case CategoryBad:
		result.Status = StatusExempt
		result.Detail = "deliberately wrong example (BAD)"
	case CategoryBash:
		result.Status = StatusExempt
		result.Detail = "bash blocks exempt by default"
	case CategoryCodeFragment:
		result.Status = StatusExempt
		result.Detail = "fragment — not compiled (needs surrounding context; covered by agent scenarios)"
	case CategoryCodeComplete:
		v.Code.Validate(result, block.Tag, doc.Content)
	case CategoryDockerfile:
		if issues := LintDockerfile(doc.Content); len(issues) > 0 {
			var details []string
			for _, issue := range issues {
				details = append(details, issue.String())
			}
			result.Status = StatusFailed
			result.Detail = "dockerfile: " + strings.Join(details, "; ")
		} else {
			result.Status = StatusValidated
		}
	case CategoryNotValidated:
		result.Status = StatusExempt
		result.Detail = "no validator for tag " + block.Tag
	case CategoryUnclassified:
		result.Status = StatusFailed
		result.Detail = "unclassified block: tag it, or annotate with <!-- eval:... -->"
	case CategoryCollectorConfig, CategoryCollectorFragment:
		v.validateCollectorDoc(result, doc.Content)
	case CategoryDockerCompose:
		var node map[string]any
		if err := yaml.Unmarshal([]byte(doc.Content), &node); err != nil {
			result.Status = StatusFailed
			result.Detail = fmt.Sprintf("docker-compose: not valid YAML: %v", err)
		} else {
			result.Status = StatusValidated
		}
	case CategoryOTTLStatements:
		if err := v.OTTL.ValidateBareOTTL(doc.Content); err != nil {
			result.Status = StatusFailed
			result.Detail = err.Error()
		} else {
			result.Status = StatusValidated
		}
	case CategoryK8sManifest:
		return v.validateK8sDoc(result, doc)
	default:
		result.Status = StatusFailed
		result.Detail = "internal: no validator for category " + string(category)
	}
	return []*Result{result}
}

// validateCollectorDoc validates one Collector configuration text: OTTL
// extraction via pkg/ottl, then otelcol-contrib validate on the (possibly
// scaffolded) configuration.
func (v *Validator) validateCollectorDoc(result *Result, content string) {
	rendered, original, scaffolded, substitutions, err := PrepareCollectorConfig(content)
	result.Substitutions = substitutions
	if err != nil {
		result.Status = StatusFailed
		result.Detail = err.Error()
		return
	}
	if scaffolded {
		result.Category = CategoryCollectorFragment
	}
	var details []string
	for _, ottlErr := range v.OTTL.ValidateCollectorOTTL(original) {
		details = append(details, "ottl: "+ottlErr.Error())
	}
	if err := v.Collector.Validate(rendered); err != nil {
		details = append(details, err.Error())
	}
	if len(details) > 0 {
		result.Status = StatusFailed
		result.Detail = strings.Join(details, "; ")
		return
	}
	result.Status = StatusValidated
}

func (v *Validator) validateK8sDoc(result *Result, doc *Document) []*Result {
	embedded, err := ParseK8sDocument(doc.Content)
	if err != nil {
		result.Status = StatusFailed
		result.Detail = "k8s manifest: " + err.Error()
		return []*Result{result}
	}
	result.Status = StatusValidated
	results := []*Result{result}
	for _, config := range embedded {
		child := &Result{
			File:       result.File,
			Line:       result.Line,
			Tag:        result.Tag,
			Category:   CategoryCollectorConfig,
			Annotation: result.Annotation,
		}
		v.validateCollectorDoc(child, config.Content)
		if child.Detail != "" {
			child.Detail = config.Source + ": " + child.Detail
		} else {
			child.Detail = config.Source
		}
		results = append(results, child)
	}
	return results
}

// Render writes a human-readable per-file report.
func (r *Report) Render() string {
	var builder strings.Builder
	builder.WriteString(r.summaryLine())
	builder.WriteString("\n\n")
	byFile := map[string][]*Result{}
	var files []string
	for _, result := range r.Results {
		if _, ok := byFile[result.File]; !ok {
			files = append(files, result.File)
		}
		byFile[result.File] = append(byFile[result.File], result)
	}
	sort.Strings(files)
	counts := map[Category]int{}
	for _, file := range files {
		fmt.Fprintf(&builder, "%s\n", file)
		for _, result := range byFile[file] {
			tag := result.Tag
			if tag == "" {
				tag = "(untagged)"
			}
			note := ""
			if result.Annotation != AnnotationNone {
				note = " [eval:" + string(result.Annotation) + "]"
			}
			detail := ""
			if result.Detail != "" {
				detail = " — " + result.Detail
			}
			subs := ""
			if len(result.Substitutions) > 0 {
				subs = " (subst: " + strings.Join(result.Substitutions, ", ") + ")"
			}
			fmt.Fprintf(&builder, "  line %-5d %-12s %-24s %s%s%s%s\n",
				result.Line, tag, result.Category, result.Status, note, detail, subs)
			counts[result.Category]++
		}
	}
	builder.WriteString("\nTotals by category:\n")
	var categories []string
	for category := range counts {
		categories = append(categories, string(category))
	}
	sort.Strings(categories)
	for _, category := range categories {
		fmt.Fprintf(&builder, "  %-26s %d\n", category, counts[Category(category)])
	}
	return builder.String()
}

// summaryLine renders a one-line headline that makes a green run legible: how
// many blocks were really checked (validated, of which code compiled) versus
// exempt or skipped. A reader can tell at a glance that a green run is not
// quietly overstating itself. The exempt breakdown lists the top exempt
// categories; skipped-no-toolchain and failed are always shown.
func (r *Report) summaryLine() string {
	byStatus := map[Status]int{}
	exemptByCategory := map[Category]int{}
	codeCompiled := 0
	classified := 0
	codeComplete := 0
	codeFragment := 0
	for _, result := range r.Results {
		byStatus[result.Status]++
		if result.Status == StatusExempt {
			exemptByCategory[result.Category]++
		}
		if result.Status == StatusValidated && result.Category == CategoryCodeComplete {
			codeCompiled++
		}
		if result.Status == StatusClassified {
			classified++
		}
		switch result.Category {
		case CategoryCodeComplete:
			codeComplete++
		case CategoryCodeFragment:
			codeFragment++
		}
	}

	// Dry-run mode records StatusClassified for every block; report the
	// classification headline (with the code split highlighted) instead of a
	// validated/exempt split that would read as all zeros.
	if classified > 0 {
		return fmt.Sprintf(
			"Summary (dry-run): %d classified, %d code-complete, %d code-fragment, %d failed.",
			classified, codeComplete, codeFragment, byStatus[StatusFailed],
		)
	}

	// Order the exempt breakdown by descending count, then category name, so
	// the biggest exemptions lead and the line is deterministic.
	type catCount struct {
		category Category
		count    int
	}
	var exemptParts []catCount
	for category, count := range exemptByCategory {
		exemptParts = append(exemptParts, catCount{category, count})
	}
	sort.Slice(exemptParts, func(i, j int) bool {
		if exemptParts[i].count != exemptParts[j].count {
			return exemptParts[i].count > exemptParts[j].count
		}
		return exemptParts[i].category < exemptParts[j].category
	})
	var breakdown []string
	for _, part := range exemptParts {
		breakdown = append(breakdown, fmt.Sprintf("%d %s", part.count, part.category))
	}
	exemptDetail := ""
	if len(breakdown) > 0 {
		exemptDetail = " (" + strings.Join(breakdown, ", ") + ")"
	}

	return fmt.Sprintf(
		"Summary: %d validated (%d code compiled), %d exempt%s, %d skipped-no-toolchain, %d failed.",
		byStatus[StatusValidated],
		codeCompiled,
		byStatus[StatusExempt],
		exemptDetail,
		byStatus[StatusSkippedNoToolchain],
		byStatus[StatusFailed],
	)
}
