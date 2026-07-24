package examples

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// collectorTopLevelKeys are the top-level sections of a Collector
// configuration.
var collectorTopLevelKeys = map[string]bool{
	"receivers":  true,
	"processors": true,
	"exporters":  true,
	"connectors": true,
	"extensions": true,
	"service":    true,
}

// ottlLineRe matches a line that starts a recognizable OTTL editor or
// converter invocation.
var ottlLineRe = regexp.MustCompile(`^\s*(set|replace_pattern|replace_all_patterns|replace_match|replace_all_matches|delete_key|delete_matching_keys|keep_keys|keep_matching_keys|limit|truncate_all|append|merge_maps|flatten|IsMatch|IsString|IsMap|IsList|IsBool|IsDouble|IsInt|Concat|SHA256|SHA1|MD5|Substring|ConvertCase|Int|Double|String|UnixNano|Now|Len|ParseJSON|ExtractPatterns|Split)\(`)

// ottlComparisonRe matches a line containing an OTTL path comparison, e.g.
// attributes["k"] != nil or time_unix_nano < UnixNano(Now()).
var ottlComparisonRe = regexp.MustCompile(`^\s*[a-zA-Z_][a-zA-Z0-9_.]*(\[[^\]]+\])*(\.[a-zA-Z0-9_.]+)?\s*(==|!=|>=|<=|>|<)\s`)

// ottlConverterRe matches a line that is a single OTTL converter invocation
// (capitalized function call), e.g. ToUpperCase(span.attributes["k"]).
var ottlConverterRe = regexp.MustCompile(`^\s*[A-Z][A-Za-z0-9_]*\(.*\)\s*$`)

// ottlPathRe matches a line that is a single OTTL path expression, e.g.
// span.name or resource.attributes["service.name"].
var ottlPathRe = regexp.MustCompile(`^\s*[a-z_][a-z0-9_]*(\.[a-z0-9_]+)+(\[[^\]]+\])?(\.[a-z0-9_]+)*(\[[^\]]+\])?\s*$`)

// yamlKeyRe matches a line that looks like a YAML mapping key.
var yamlKeyRe = regexp.MustCompile(`^\s*[A-Za-z_][\w./-]*:(\s|$)`)

// Classify assigns a validation category to a document. The block's
// annotation wins; otherwise heuristics decide. The eval:fragment annotation
// is context-aware: on a code-tagged block it forces CategoryCodeFragment, on
// a yaml or untagged block it forces CategoryCollectorFragment.
func Classify(doc *Document) Category {
	b := doc.Block
	switch b.Annotation {
	case AnnotationSkip:
		return CategorySkip
	case AnnotationBad:
		return CategoryBad
	case AnnotationCollectorConfig:
		return CategoryCollectorConfig
	case AnnotationFragment:
		if codeTags[b.Tag] {
			return CategoryCodeFragment
		}
		return CategoryCollectorFragment
	case AnnotationK8s:
		return CategoryK8sManifest
	}
	if b.HasBadMarker() {
		return CategoryBad
	}
	switch b.Tag {
	case "bash":
		return CategoryBash
	case "yaml", "":
		return classifyYAMLOrUntagged(doc)
	}
	if codeTags[b.Tag] {
		if isCompleteCode(b.Tag, doc.Content) {
			return CategoryCodeComplete
		}
		return CategoryCodeFragment
	}
	return CategoryNotValidated
}

func classifyYAMLOrUntagged(doc *Document) Category {
	var node map[string]any
	if err := yaml.Unmarshal([]byte(doc.Content), &node); err == nil && node != nil {
		if _, ok := node["apiVersion"]; ok {
			return CategoryK8sManifest
		}
		collectorKeys := 0
		for key := range node {
			if collectorTopLevelKeys[key] {
				collectorKeys++
			}
		}
		if collectorKeys > 0 && collectorKeys == len(node) {
			if isCompleteCollectorConfig(node) {
				return CategoryCollectorConfig
			}
			return CategoryCollectorFragment
		}
		if isDockerCompose(node) {
			return CategoryDockerCompose
		}
	}
	if isOTTLBlock(doc.Content) {
		return CategoryOTTLStatements
	}
	return CategoryUnclassified
}

// isCompleteCollectorConfig reports whether the parsed Collector config has
// a service section with at least one pipeline and defines every component
// its pipelines reference.
func isCompleteCollectorConfig(node map[string]any) bool {
	pipelines := collectorPipelines(node)
	if len(pipelines) == 0 {
		return false
	}
	return len(missingComponents(node)) == 0
}

func isDockerCompose(node map[string]any) bool {
	services, ok := node["services"].(map[string]any)
	if !ok || len(services) == 0 {
		return false
	}
	for _, service := range services {
		spec, ok := service.(map[string]any)
		if !ok {
			return false
		}
		if _, hasImage := spec["image"]; hasImage {
			continue
		}
		if _, hasBuild := spec["build"]; hasBuild {
			continue
		}
		return false
	}
	return true
}

// isOTTLBlock reports whether every non-empty, non-comment line looks like
// an OTTL statement, condition, or condition continuation, with at least one
// strong OTTL line present.
func isOTTLBlock(content string) bool {
	strong := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if yamlKeyRe.MatchString(line) {
			return false
		}
		switch {
		case ottlLineRe.MatchString(line):
			strong = true
		case ottlComparisonRe.MatchString(line):
			strong = true
		case ottlConverterRe.MatchString(line):
			strong = true
		case ottlPathRe.MatchString(line):
			strong = true
		case trimmed == "and" || trimmed == "or" || trimmed == "not":
			// Boolean connective between condition lines.
		case strings.HasPrefix(trimmed, "where "):
			// Continuation of a wrapped statement.
		default:
			return false
		}
	}
	return strong
}
