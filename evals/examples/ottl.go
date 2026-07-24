package examples

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component/componenttest"
)

// ottlParser parses OTTL statements, conditions, and value expressions
// for one context.
type ottlParser struct {
	name           string
	parseStatement func(statement string) error
	parseCondition func(condition string) error
	parseValueExpr func(expression string) error
}

func wrapParser[K any](name string, parser ottl.Parser[K]) *ottlParser {
	return &ottlParser{
		name:           name,
		parseStatement: func(s string) error { _, err := parser.ParseStatement(s); return err },
		parseCondition: func(c string) error { _, err := parser.ParseCondition(c); return err },
		parseValueExpr: func(e string) error { _, err := parser.ParseValueExpression(e); return err },
	}
}

// buildParsers constructs one parser per OTTL context with StandardFuncs.
func buildParsers() (map[string]*ottlParser, error) {
	settings := componenttest.NewNopTelemetrySettings()
	parsers := map[string]*ottlParser{}

	spanParser, err := ottlspan.NewParser(ottlfuncs.StandardFuncs[*ottlspan.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["span"] = wrapParser("span", spanParser)

	spanEventParser, err := ottlspanevent.NewParser(ottlfuncs.StandardFuncs[*ottlspanevent.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["spanevent"] = wrapParser("spanevent", spanEventParser)

	logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["log"] = wrapParser("log", logParser)

	metricParser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[*ottlmetric.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["metric"] = wrapParser("metric", metricParser)

	datapointParser, err := ottldatapoint.NewParser(ottlfuncs.StandardFuncs[*ottldatapoint.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["datapoint"] = wrapParser("datapoint", datapointParser)

	resourceParser, err := ottlresource.NewParser(ottlfuncs.StandardFuncs[*ottlresource.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["resource"] = wrapParser("resource", resourceParser)

	scopeParser, err := ottlscope.NewParser(ottlfuncs.StandardFuncs[*ottlscope.TransformContext](), settings)
	if err != nil {
		return nil, err
	}
	parsers["scope"] = wrapParser("scope", scopeParser)

	return parsers, nil
}

// OTTLValidator parses OTTL statements and conditions against their signal
// contexts via pkg/ottl (pinned to the same version as otelcol-contrib).
type OTTLValidator struct {
	parsers map[string]*ottlParser
}

// NewOTTLValidator builds parsers for every supported OTTL context.
func NewOTTLValidator() (*OTTLValidator, error) {
	parsers, err := buildParsers()
	if err != nil {
		return nil, fmt.Errorf("ottl: build parsers: %w", err)
	}
	return &OTTLValidator{parsers: parsers}, nil
}

// unescapeConfmap reverses the confmap $$ escaping the Collector applies
// before handing statements to the OTTL parser.
func unescapeConfmap(statement string) string {
	return strings.ReplaceAll(statement, "$$", "$")
}

// parseStatementIn tries the statement in each named context; nil when any
// parses.
func (v *OTTLValidator) parseStatementIn(statement string, contexts []string) error {
	var firstErr error
	for _, name := range contexts {
		parser, ok := v.parsers[name]
		if !ok {
			continue
		}
		err := parser.parseStatement(statement)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("context %s: %w", name, err)
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("no OTTL context available among %v", contexts)
	}
	return firstErr
}

// transformGroupContexts maps a transform statement-group key to the
// contexts its flat-form statements may target, most specific first.
var transformGroupContexts = map[string][]string{
	"trace_statements":  {"span", "spanevent", "resource", "scope"},
	"log_statements":    {"log", "resource", "scope"},
	"metric_statements": {"metric", "datapoint", "resource", "scope"},
	"profile_statements": {
		"resource", "scope",
	},
}

// filterConditionContexts maps filter processor condition list keys to
// their OTTL context.
var filterConditionContexts = map[string]string{
	"span":       "span",
	"spanevent":  "spanevent",
	"metric":     "metric",
	"datapoint":  "datapoint",
	"log_record": "log",
}

// ValidateCollectorOTTL extracts and parses every OTTL statement and
// condition from transform and filter processors in a parsed Collector
// configuration. It returns one error per failing statement.
func (v *OTTLValidator) ValidateCollectorOTTL(node map[string]any) []error {
	var errs []error
	processors := section(node, "processors")
	for _, id := range sortedKeys(processors) {
		config, _ := processors[id].(map[string]any)
		switch componentType(id) {
		case "transform":
			errs = append(errs, v.validateTransform(id, config)...)
		case "filter":
			errs = append(errs, v.validateFilter(id, config)...)
		}
	}
	return errs
}

func (v *OTTLValidator) validateTransform(id string, config map[string]any) []error {
	var errs []error
	appendErr := func(group string, err error) {
		errs = append(errs, fmt.Errorf("processor %s, %s: %v", id, group, err))
	}
	// Flat context-inferred form: statements directly under the processor.
	if statements, ok := config["statements"].([]any); ok {
		for _, statementValue := range statements {
			statement, ok := statementValue.(string)
			if !ok {
				continue
			}
			allContexts := []string{"span", "spanevent", "log", "metric", "datapoint", "resource", "scope"}
			if err := v.parseStatementIn(unescapeConfmap(statement), allContexts); err != nil {
				appendErr("statements", err)
			}
		}
	}
	for group, flatContexts := range transformGroupContexts {
		groupList, ok := config[group].([]any)
		if !ok {
			continue
		}
		for _, element := range groupList {
			switch element := element.(type) {
			case string:
				// Flat form: context inferred from the statement paths.
				if err := v.parseStatementIn(unescapeConfmap(element), flatContexts); err != nil {
					appendErr(group, err)
				}
			case map[string]any:
				errs = append(errs, v.validateStatementGroup(id, group, element, flatContexts)...)
			}
		}
	}
	return errs
}

func (v *OTTLValidator) validateStatementGroup(id, group string, element map[string]any, flatContexts []string) []error {
	var errs []error
	contextName, _ := element["context"].(string)
	contexts := flatContexts
	if contextName != "" {
		contexts = []string{contextName}
	}
	parser := v.parsers[contextName]
	if statements, ok := element["statements"].([]any); ok {
		for _, statementValue := range statements {
			statement, ok := statementValue.(string)
			if !ok {
				continue
			}
			if err := v.parseStatementIn(unescapeConfmap(statement), contexts); err != nil {
				errs = append(errs, fmt.Errorf("processor %s, %s: %v", id, group, err))
			}
		}
	}
	if conditions, ok := element["conditions"].([]any); ok && parser != nil {
		for _, conditionValue := range conditions {
			condition, ok := conditionValue.(string)
			if !ok {
				continue
			}
			if err := parser.parseCondition(unescapeConfmap(condition)); err != nil {
				errs = append(errs, fmt.Errorf("processor %s, %s conditions: %v", id, group, err))
			}
		}
	}
	return errs
}

func (v *OTTLValidator) validateFilter(id string, config map[string]any) []error {
	var errs []error
	for _, signalKey := range []string{"traces", "metrics", "logs"} {
		signalConfig, ok := config[signalKey].(map[string]any)
		if !ok {
			continue
		}
		for listKey, contextName := range filterConditionContexts {
			conditions, ok := signalConfig[listKey].([]any)
			if !ok {
				continue
			}
			parser, ok := v.parsers[contextName]
			if !ok {
				continue
			}
			for _, conditionValue := range conditions {
				condition, ok := conditionValue.(string)
				if !ok {
					continue
				}
				if err := parser.parseCondition(unescapeConfmap(condition)); err != nil {
					errs = append(errs, fmt.Errorf("processor %s, %s.%s: %v", id, signalKey, listKey, err))
				}
			}
		}
	}
	return errs
}

// bareContexts is the context order tried for bare ottl-statements blocks:
// span first, then fallback attempts across the other contexts.
var bareContexts = []string{"span", "log", "metric", "datapoint", "spanevent", "resource", "scope"}

// ValidateBareOTTL validates a bare block of OTTL text. The block passes
// when the whole block parses as one condition in some context, or when
// every line individually parses as a statement, condition, or value
// expression in some context (span tried first, with fallback attempts
// across the other contexts).
func (v *OTTLValidator) ValidateBareOTTL(content string) error {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		lines = append(lines, unescapeConfmap(trimmed))
	}
	if len(lines) == 0 {
		return nil
	}
	// Multi-line conditions (connectives on their own lines) parse as one
	// condition.
	whole := strings.Join(lines, " ")
	for _, name := range bareContexts {
		if v.parsers[name].parseCondition(whole) == nil {
			return nil
		}
	}
	for _, line := range lines {
		if err := v.parseLineAnyContext(line); err != nil {
			return fmt.Errorf("line %q: %v", line, err)
		}
	}
	return nil
}

// parseLineAnyContext parses one bare OTTL line as a statement, condition,
// or value expression in any context.
func (v *OTTLValidator) parseLineAnyContext(line string) error {
	var firstErr error
	for _, name := range bareContexts {
		parser := v.parsers[name]
		err := parser.parseStatement(line)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("context %s: %v", name, err)
		}
		if parser.parseCondition(line) == nil {
			return nil
		}
		if parser.parseValueExpr(line) == nil {
			return nil
		}
	}
	return firstErr
}
