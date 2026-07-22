package examples

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// signalSet marks which pipeline signal types a component supports.
type signalSet struct{ traces, metrics, logs bool }

var allSignals = signalSet{traces: true, metrics: true, logs: true}

// componentType returns the type part of a component ID (before the first
// "/").
func componentType(id string) string {
	componentType, _, _ := strings.Cut(id, "/")
	return componentType
}

// receiverSignals maps receiver types to the signals they emit. Unknown
// types default to all signals; otelcol validate does not type-check
// pipelines, so the default only shapes the generated scaffold.
var receiverSignals = map[string]signalSet{
	"jaeger":        {traces: true},
	"zipkin":        {traces: true},
	"prometheus":    {metrics: true},
	"hostmetrics":   {metrics: true},
	"kubeletstats":  {metrics: true},
	"k8s_cluster":   {metrics: true},
	"filelog":       {logs: true},
	"fluentforward": {logs: true},
	"k8s_events":    {logs: true},
}

// processorSignals maps processor types to the signals they support.
var processorSignals = map[string]signalSet{
	"tail_sampling":         {traces: true},
	"span":                  {traces: true},
	"probabilistic_sampler": {traces: true, logs: true},
	"metricstransform":      {metrics: true},
	"cumulativetodelta":     {metrics: true},
	"deltatocumulative":     {metrics: true},
	"logdedup":              {logs: true},
}

// exporterSignals maps exporter types to the signals they accept.
var exporterSignals = map[string]signalSet{
	"prometheus":            {metrics: true},
	"prometheusremotewrite": {metrics: true},
}

// connectorSignals maps connector types to their (source, target) pipeline
// signals. Unknown connectors default to traces → metrics.
var connectorSignals = map[string][2]string{
	"signaltometrics": {"traces", "metrics"},
	"spanmetrics":     {"traces", "metrics"},
	"count":           {"traces", "metrics"},
	"sum":             {"traces", "metrics"},
	"servicegraph":    {"traces", "metrics"},
	"exceptions":      {"traces", "logs"},
	"routing":         {"traces", "traces"},
	"forward":         {"traces", "traces"},
	"failover":        {"traces", "traces"},
	"roundrobin":      {"traces", "traces"},
}

func lookupSignals(table map[string]signalSet, id string) signalSet {
	if signals, ok := table[componentType(id)]; ok {
		return signals
	}
	return allSignals
}

func (s signalSet) has(signal string) bool {
	switch signal {
	case "traces":
		return s.traces
	case "metrics":
		return s.metrics
	case "logs":
		return s.logs
	}
	return false
}

// stubConfigs provides minimal valid configurations for components the
// scaffold generator defines when a fragment references them without
// defining them. Types not listed stub with an empty configuration.
var stubConfigs = map[string]func() any{
	"receivers/otlp": func() any {
		return map[string]any{"protocols": map[string]any{"grpc": map[string]any{"endpoint": "127.0.0.1:4317"}}}
	},
	"exporters/otlp": func() any {
		return map[string]any{"endpoint": "https://collector.example.invalid:4317"}
	},
	"exporters/otlphttp": func() any {
		return map[string]any{"endpoint": "https://collector.example.invalid:4318"}
	},
	"processors/memory_limiter": func() any {
		return map[string]any{"check_interval": "1s", "limit_mib": 512}
	},
	"processors/resource": func() any {
		return map[string]any{"attributes": []any{map[string]any{"key": "eval.scaffold", "value": "true", "action": "upsert"}}}
	},
	"processors/attributes": func() any {
		return map[string]any{"actions": []any{map[string]any{"key": "eval.scaffold", "value": "true", "action": "upsert"}}}
	},
	"processors/tail_sampling": func() any {
		return map[string]any{"policies": []any{map[string]any{"name": "eval", "type": "always_sample"}}}
	},
	"extensions/file_storage": func() any {
		return map[string]any{"directory": os.TempDir()}
	},
	"connectors/signaltometrics": func() any {
		return map[string]any{"spans": []any{map[string]any{
			"name":        "eval.scaffold.count",
			"description": "eval scaffold",
			"unit":        "1",
			"sum":         map[string]any{"value": "Int(1)"},
		}}}
	},
}

func stubConfig(section, id string) any {
	if factory, ok := stubConfigs[section+"/"+componentType(id)]; ok {
		return factory()
	}
	return nil
}

// section returns the named top-level section as a map, or nil.
func section(node map[string]any, name string) map[string]any {
	sectionMap, _ := node[name].(map[string]any)
	return sectionMap
}

// collectorPipelines returns service.pipelines, or nil.
func collectorPipelines(node map[string]any) map[string]any {
	return section(section(node, "service"), "pipelines")
}

// missingComponents returns "section/id" strings for every component a
// pipeline references without a matching definition.
func missingComponents(node map[string]any) []string {
	var missing []string
	connectors := section(node, "connectors")
	seen := map[string]bool{}
	for _, pipeline := range collectorPipelines(node) {
		pipelineMap, ok := pipeline.(map[string]any)
		if !ok {
			continue
		}
		for _, sectionName := range []string{"receivers", "processors", "exporters"} {
			ids, _ := pipelineMap[sectionName].([]any)
			defined := section(node, sectionName)
			for _, idValue := range ids {
				id, ok := idValue.(string)
				if !ok {
					continue
				}
				if _, isDefined := defined[id]; isDefined {
					continue
				}
				// Connectors appear as pipeline receivers and exporters.
				if sectionName != "processors" {
					if _, isConnector := connectors[id]; isConnector {
						continue
					}
				}
				key := sectionName + "/" + id
				if !seen[key] {
					seen[key] = true
					missing = append(missing, key)
				}
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ScaffoldCollectorConfig completes a Collector configuration fragment so
// that otelcol validate can run against it. It generates service pipelines
// when none exist (wiring a stub otlp receiver and debug exporter around the
// fragment's components, and paired pipelines around connectors), and stubs
// definitions for components that pipelines reference without defining.
// It returns whether the configuration was modified.
func ScaffoldCollectorConfig(node map[string]any) bool {
	changed := false
	ensureSection := func(name string) map[string]any {
		if sectionMap := section(node, name); sectionMap != nil {
			return sectionMap
		}
		sectionMap := map[string]any{}
		node[name] = sectionMap
		return sectionMap
	}
	define := func(sectionName, id string) {
		sectionMap := ensureSection(sectionName)
		if _, ok := sectionMap[id]; !ok {
			sectionMap[id] = stubConfig(sectionName, id)
			changed = true
		}
	}

	if len(collectorPipelines(node)) == 0 {
		changed = true
		generatePipelines(node, define)
	}

	// Stub every referenced-but-undefined component. IDs whose type is a
	// known connector type (referenced as pipeline receivers and
	// exporters) are defined as connectors.
	for _, missing := range missingComponents(node) {
		sectionName, id, _ := strings.Cut(missing, "/")
		if sectionName != "processors" {
			if _, isConnectorType := connectorSignals[componentType(id)]; isConnectorType {
				define("connectors", id)
				continue
			}
		}
		define(sectionName, id)
	}

	// Every defined extension joins service.extensions; every listed
	// extension gets a definition.
	if extensions := section(node, "extensions"); len(extensions) > 0 {
		service, _ := node["service"].(map[string]any)
		if service == nil {
			service = map[string]any{}
			node["service"] = service
		}
		listed, _ := service["extensions"].([]any)
		present := map[string]bool{}
		for _, id := range listed {
			if name, ok := id.(string); ok {
				present[name] = true
			}
		}
		for _, id := range sortedKeys(extensions) {
			if !present[id] {
				listed = append(listed, id)
				changed = true
			}
		}
		service["extensions"] = listed
	}
	if service := section(node, "service"); service != nil {
		if listed, ok := service["extensions"].([]any); ok {
			for _, id := range listed {
				if name, ok := id.(string); ok {
					define("extensions", name)
				}
			}
		}
	}
	return changed
}

// generatePipelines builds service pipelines exercising every defined
// component of a service-less fragment.
func generatePipelines(node map[string]any, define func(section, id string)) {
	receivers := section(node, "receivers")
	processors := section(node, "processors")
	exporters := section(node, "exporters")
	connectors := section(node, "connectors")

	// Determine which signals the fragment's components need.
	needed := map[string]bool{}
	for _, id := range sortedKeys(receivers) {
		signals := lookupSignals(receiverSignals, id)
		for _, signal := range []string{"traces", "metrics", "logs"} {
			if signals.has(signal) {
				needed[signal] = true
			}
		}
	}
	for _, id := range sortedKeys(processors) {
		signals := lookupSignals(processorSignals, id)
		if signals == allSignals {
			continue // signal-agnostic processors do not force pipelines
		}
		for _, signal := range []string{"traces", "metrics", "logs"} {
			if signals.has(signal) {
				needed[signal] = true
			}
		}
	}
	for _, id := range sortedKeys(exporters) {
		signals := lookupSignals(exporterSignals, id)
		if signals == allSignals {
			continue
		}
		for _, signal := range []string{"traces", "metrics", "logs"} {
			if signals.has(signal) {
				needed[signal] = true
			}
		}
	}
	if len(needed) == 0 && (len(receivers) > 0 || len(processors) > 0 || len(exporters) > 0) {
		needed["traces"] = true
	}

	pipelines := map[string]any{}
	buildPipeline := func(signal string) map[string]any {
		var pipelineReceivers, pipelineProcessors, pipelineExporters []any
		for _, id := range sortedKeys(receivers) {
			if lookupSignals(receiverSignals, id).has(signal) {
				pipelineReceivers = append(pipelineReceivers, id)
			}
		}
		if len(pipelineReceivers) == 0 {
			define("receivers", "otlp")
			pipelineReceivers = append(pipelineReceivers, "otlp")
		}
		for _, id := range sortedKeys(processors) {
			if lookupSignals(processorSignals, id).has(signal) {
				pipelineProcessors = append(pipelineProcessors, id)
			}
		}
		for _, id := range sortedKeys(exporters) {
			if lookupSignals(exporterSignals, id).has(signal) {
				pipelineExporters = append(pipelineExporters, id)
			}
		}
		if len(pipelineExporters) == 0 {
			define("exporters", "debug")
			pipelineExporters = append(pipelineExporters, "debug")
		}
		pipeline := map[string]any{
			"receivers": pipelineReceivers,
			"exporters": pipelineExporters,
		}
		if len(pipelineProcessors) > 0 {
			pipeline["processors"] = pipelineProcessors
		}
		return pipeline
	}
	for _, signal := range []string{"traces", "metrics", "logs"} {
		if needed[signal] {
			pipelines[signal] = buildPipeline(signal)
		}
	}

	// Connectors get a source pipeline exporting to them and a target
	// pipeline receiving from them.
	for _, id := range sortedKeys(connectors) {
		signals, ok := connectorSignals[componentType(id)]
		if !ok {
			signals = [2]string{"traces", "metrics"}
		}
		source, target := signals[0], signals[1]
		sourcePipeline, ok := pipelines[source].(map[string]any)
		if !ok {
			sourcePipeline = buildPipeline(source)
			pipelines[source] = sourcePipeline
		}
		sourceExporters, _ := sourcePipeline["exporters"].([]any)
		sourcePipeline["exporters"] = append(sourceExporters, id)

		define("exporters", "debug")
		pipelines[target+"/eval-"+strings.ReplaceAll(id, "/", "-")] = map[string]any{
			"receivers": []any{id},
			"exporters": []any{"debug"},
		}
	}

	if len(pipelines) == 0 {
		// Extensions-only fragment: a minimal pipeline keeps the service
		// section valid.
		define("receivers", "otlp")
		define("exporters", "debug")
		pipelines["traces"] = map[string]any{
			"receivers": []any{"otlp"},
			"exporters": []any{"debug"},
		}
	}

	service, _ := node["service"].(map[string]any)
	if service == nil {
		service = map[string]any{}
		node["service"] = service
	}
	service["pipelines"] = pipelines
}

// retargetHostPaths points host-dependent paths in the configuration at
// paths that exist on the validating host, so environment checks (for
// example the file_storage directory-must-exist check) do not fail on
// paths that only exist in the deployed environment.
func retargetHostPaths(node map[string]any) []string {
	var substitutions []string
	extensions := section(node, "extensions")
	for _, id := range sortedKeys(extensions) {
		if componentType(id) != "file_storage" {
			continue
		}
		config, _ := extensions[id].(map[string]any)
		if config == nil {
			continue
		}
		if directory, ok := config["directory"].(string); ok {
			config["directory"] = os.TempDir()
			substitutions = append(substitutions, fmt.Sprintf("extensions.%s.directory %s -> %s", id, directory, os.TempDir()))
		}
	}
	return substitutions
}

var placeholderRe = regexp.MustCompile(`<([A-Z][A-Z0-9_]{2,})>`)

// endpointKeyRe matches a YAML key whose value should be endpoint-shaped.
var endpointKeyRe = regexp.MustCompile(`(?i)(^|\s)([\w.-]*(endpoint|url))\s*:`)

// SubstitutePlaceholders replaces <UPPER_SNAKE> placeholder tokens with
// syntactically valid dummy values: endpoint-shaped values for endpoint or
// URL keys, opaque strings elsewhere. It returns the substituted text and a
// description of each substitution.
func SubstitutePlaceholders(content string) (string, []string) {
	var substitutions []string
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !placeholderRe.MatchString(line) {
			continue
		}
		endpointShaped := endpointKeyRe.MatchString(line)
		lines[i] = placeholderRe.ReplaceAllStringFunc(line, func(match string) string {
			name := placeholderRe.FindStringSubmatch(match)[1]
			var value string
			if endpointShaped {
				value = "https://eval-dummy.invalid:4317"
			} else {
				value = "eval-dummy-" + strings.ToLower(strings.ReplaceAll(name, "_", "-"))
			}
			substitutions = append(substitutions, fmt.Sprintf("<%s> -> %s", name, value))
			return value
		})
	}
	return strings.Join(lines, "\n"), substitutions
}

var envRefRe = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// dummyEnv returns the fixed dummy variables plus one for every ${env:...}
// reference discovered in the configuration, so env-reference expansion
// cannot fail on unset variables.
func dummyEnv(content string) []string {
	values := map[string]string{
		"DASH0_AUTH_TOKEN": "eval-dummy-token",
		"DASH0_TOKEN":      "eval-dummy-token",
	}
	for _, match := range envRefRe.FindAllStringSubmatch(content, -1) {
		if _, ok := values[match[1]]; !ok {
			values[match[1]] = "eval-dummy"
		}
	}
	env := make([]string, 0, len(values))
	for _, key := range sortedKeysString(values) {
		env = append(env, key+"="+values[key])
	}
	return env
}

func sortedKeysString(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// CollectorValidator validates Collector configurations with a pinned
// otelcol-contrib binary.
type CollectorValidator struct {
	// BinaryPath is the path to the otelcol-contrib binary.
	BinaryPath string
	// Timeout bounds one validate invocation. Zero means 30 seconds.
	Timeout time.Duration
}

// Validate runs `otelcol-contrib validate --config <file>` against the
// given configuration text with dummy environment variables exported.
func (v *CollectorValidator) Validate(configYAML string) error {
	dir, err := os.MkdirTemp("", "eval-otelcol-*")
	if err != nil {
		return fmt.Errorf("collector validate: %w", err)
	}
	defer os.RemoveAll(dir)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		return fmt.Errorf("collector validate: %w", err)
	}
	timeout := v.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, v.BinaryPath, "validate", "--config", configPath)
	cmd.Env = append(os.Environ(), dummyEnv(configYAML)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("otelcol-contrib validate failed: %s", condenseOutput(string(output)))
	}
	return nil
}

// condenseOutput trims a validate error message to its informative lines.
func condenseOutput(output string) string {
	var kept []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kept = append(kept, line)
	}
	const maxLines = 4
	if len(kept) > maxLines {
		kept = kept[:maxLines]
	}
	return strings.Join(kept, " | ")
}

// PrepareCollectorConfig substitutes placeholders, parses, and scaffolds a
// Collector configuration document. It returns the ready-to-validate YAML,
// the parsed original document (for OTTL extraction), whether scaffolding
// modified the configuration, and the placeholder substitutions applied.
func PrepareCollectorConfig(content string) (string, map[string]any, bool, []string, error) {
	substituted, substitutions := SubstitutePlaceholders(content)
	var node map[string]any
	if err := yaml.Unmarshal([]byte(substituted), &node); err != nil {
		return "", nil, false, substitutions, fmt.Errorf("not valid YAML: %v", err)
	}
	if node == nil {
		return "", nil, false, substitutions, fmt.Errorf("empty configuration")
	}
	var original map[string]any
	_ = yaml.Unmarshal([]byte(substituted), &original)
	substitutions = append(substitutions, retargetHostPaths(node)...)
	scaffolded := ScaffoldCollectorConfig(node)
	rendered, err := yaml.Marshal(node)
	if err != nil {
		return "", nil, false, substitutions, fmt.Errorf("render scaffolded config: %v", err)
	}
	return string(rendered), original, scaffolded, substitutions, nil
}
