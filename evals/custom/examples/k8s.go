package examples

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmbeddedCollectorConfig is a Collector configuration found inside a
// Kubernetes manifest.
type EmbeddedCollectorConfig struct {
	// Source describes where the configuration was embedded (for example
	// "OpenTelemetryCollector spec.config" or "ConfigMap data[config.yaml]").
	Source string
	// Content is the embedded configuration as YAML text.
	Content string
}

// ParseK8sDocument parses a Kubernetes manifest document and extracts any
// embedded Collector configuration: OpenTelemetryCollector CR spec.config
// (string or structured) and ConfigMap data values that look like Collector
// configuration.
func ParseK8sDocument(content string) ([]EmbeddedCollectorConfig, error) {
	// Placeholders like <OTLP_ENDPOINT> are plain YAML scalars, so the
	// manifest parses as written; embedded Collector configurations keep
	// their placeholders and substitute them during their own validation.
	var node map[string]any
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		return nil, fmt.Errorf("not valid YAML: %v", err)
	}
	if node == nil {
		return nil, fmt.Errorf("empty document")
	}
	kind, _ := node["kind"].(string)
	switch kind {
	case "OpenTelemetryCollector":
		return extractCRConfig(node)
	case "ConfigMap":
		return extractConfigMapConfigs(node), nil
	}
	return nil, nil
}

func extractCRConfig(node map[string]any) ([]EmbeddedCollectorConfig, error) {
	spec := section(node, "spec")
	if spec == nil {
		return nil, nil
	}
	source := "OpenTelemetryCollector spec.config"
	switch config := spec["config"].(type) {
	case nil:
		return nil, nil
	case string:
		return []EmbeddedCollectorConfig{{Source: source, Content: config}}, nil
	case map[string]any:
		rendered, err := yaml.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("render %s: %v", source, err)
		}
		return []EmbeddedCollectorConfig{{Source: source, Content: string(rendered)}}, nil
	default:
		return nil, fmt.Errorf("%s has unexpected type %T", source, config)
	}
}

func extractConfigMapConfigs(node map[string]any) []EmbeddedCollectorConfig {
	data := section(node, "data")
	var embedded []EmbeddedCollectorConfig
	for _, key := range sortedKeys(data) {
		value, ok := data[key].(string)
		if !ok || !isCollectorConfigValue(key, value) {
			continue
		}
		embedded = append(embedded, EmbeddedCollectorConfig{
			Source:  fmt.Sprintf("ConfigMap data[%s]", key),
			Content: value,
		})
	}
	return embedded
}

// collectorSectionLineRe matches a top-level Collector configuration section
// header (unindented mapping key), so a syntactically broken value that is
// nonetheless intended as Collector configuration is still recognized and
// validated rather than silently skipped.
var collectorSectionLineRe = regexp.MustCompile(`(?m)^(receivers|processors|exporters|connectors|extensions|service):`)

// isCollectorConfigValue reports whether a ConfigMap data value should be
// extracted and validated as Collector configuration. A value qualifies when
// its key carries a YAML file extension, or its content looks like Collector
// configuration; the latter recognizes both cleanly parsing configs and
// syntactically broken ones that carry top-level Collector sections, so a
// syntax error inside them fails validation instead of escaping it.
func isCollectorConfigValue(key, content string) bool {
	if hasYAMLExtension(key) {
		return true
	}
	return looksLikeCollectorConfig(content)
}

// hasYAMLExtension reports whether the key ends in .yaml or .yml.
func hasYAMLExtension(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// looksLikeCollectorConfig reports whether the text is Collector
// configuration. When the text parses as a YAML map, every top-level key must
// be a Collector section. When it does not parse (a syntax error), it still
// counts as Collector configuration if it carries a top-level Collector
// section header, so the broken value reaches validation.
func looksLikeCollectorConfig(content string) bool {
	var node map[string]any
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		return collectorSectionLineRe.MatchString(content)
	}
	if len(node) == 0 {
		return false
	}
	for key := range node {
		if !collectorTopLevelKeys[key] {
			return false
		}
	}
	return true
}
