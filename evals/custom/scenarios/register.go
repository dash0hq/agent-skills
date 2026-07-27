// Scenario self-registration: each implementation unit declares its scenario
// set in its own file and registers it from an init function, so units land
// additively without editing scenarios.go (and without merge conflicts
// between units developed in parallel).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

// extraScenarios collects the per-unit scenario sets registered via
// registerScenarios. All() (see scenarios.go) appends it to the base
// scenario list. Registration order is deterministic: within one file it is
// the call order, across files it follows the package's file initialization
// order (lexical by file name).
var extraScenarios []harness.Scenario

// registerScenarios appends scenarios to the extra scenario set. Call it from
// an init function of the unit's scenario file; duplicate IDs are rejected by
// Registry.Register when Default() builds the populated registry.
func registerScenarios(scs ...harness.Scenario) {
	extraScenarios = append(extraScenarios, scs...)
}
