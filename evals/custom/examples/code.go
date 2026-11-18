package examples

import (
	"fmt"
)

// CodeValidator compiles complete SDK code blocks. Blocks of a language with
// a container plug-in (see ContainerLanguage) are queued on Containers and
// compiled per rule file by Finish; blocks in every other language are
// reported skipped-no-toolchain. The zero value is not usable; build one with
// NewCodeValidator.
type CodeValidator struct {
	// goCoreVersion pins the stable OpenTelemetry Go modules
	// (OTEL_GO_CORE_VERSION).
	goCoreVersion string
	// goLogVersion pins the pre-release OpenTelemetry Go log modules
	// (OTEL_GO_LOG_VERSION).
	goLogVersion string

	// Containers compiles the languages with a container plug-in; when nil,
	// their complete blocks report skipped-no-toolchain.
	Containers *ContainerRunner
}

// NewCodeValidator builds a CodeValidator with the pinned OpenTelemetry Go
// versions, which Finish hands to the Go container plug-in.
func NewCodeValidator(goCoreVersion, goLogVersion string) *CodeValidator {
	return &CodeValidator{
		goCoreVersion: goCoreVersion,
		goLogVersion:  goLogVersion,
	}
}

// Validate records the outcome of one code-complete block on result. It
// routes by the block's normalized tag: a container language is queued until
// Finish, and every other language reports StatusSkippedNoToolchain.
func (c *CodeValidator) Validate(result *Result, block *Block) {
	if c.Containers != nil && c.Containers.Handles(block.Tag) {
		c.Containers.Enqueue(result, block)
		return
	}
	result.Status = StatusSkippedNoToolchain
	result.Detail = fmt.Sprintf("no %s compiler wired (no container plug-in for the language yet); complete block not compiled", block.Tag)
}

// Finish compiles the queued container-language blocks, one container per
// rule file and language, with the Go plug-in pinned to the validator's
// OpenTelemetry Go versions. all is every block of the validated tree.
func (c *CodeValidator) Finish(all []*Block) {
	if c.Containers != nil {
		c.Containers.Register(NewGoLanguage(c.goCoreVersion, c.goLogVersion))
		c.Containers.Run(all)
	}
}
