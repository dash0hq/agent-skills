package examples

import "regexp"

// The completeness detectors decide whether an SDK code block is a
// self-contained compilation unit (code-complete) or an intentional snippet
// (code-fragment). Snippets dominate skill content: import blocks, method
// bodies, and elided examples. The detectors are deliberately conservative —
// when a block is ambiguous they return false so it is treated as a fragment,
// because a false "complete" wrongly fails a run while a false "fragment" only
// skips a compile. Non-Go complete blocks are compile-skipped anyway (their
// fixture-image compilers are a follow-up), so only the Go detector needs to
// be precise today; the other heuristics are simple and bias to fragment.

// goPackageRe matches a top-level Go package declaration at column 0.
var goPackageRe = regexp.MustCompile(`(?m)^package\s+\w+`)

// pyTopLevelRe matches a Python top-level import, def, or class at column 0.
var pyTopLevelRe = regexp.MustCompile(`(?m)^(import\s+\w|from\s+\w|def\s+\w|class\s+\w)`)

// javaTypeRe matches a Java top-level class, interface, or enum at column 0.
var javaTypeRe = regexp.MustCompile(`(?m)^(public\s+|final\s+|abstract\s+)*(class|interface|enum)\s+\w`)

// csharpTypeRe matches a C# top-level namespace, class, interface, or struct.
var csharpTypeRe = regexp.MustCompile(`(?m)^(namespace|(public\s+|internal\s+|sealed\s+|static\s+)*(class|interface|struct))\s+\w`)

// jsEntrypointRe matches a JavaScript or TypeScript top-level import or
// require at column 0, a rough proxy for a runnable entrypoint.
var jsEntrypointRe = regexp.MustCompile(`(?m)^(import\s|const\s+\w+\s*=\s*require\()`)

// isCompleteCode reports whether a code block of the given normalized tag is a
// self-contained compilation unit. It never panics on unknown tags: an
// unrecognized tag returns false (fragment).
func isCompleteCode(tag, content string) bool {
	switch tag {
	case "go":
		// A Go source file is a compilation unit iff it declares a package.
		return goPackageRe.MatchString(content)
	case "python":
		// Bias to fragment: require a top-level import, def, or class, which
		// most snippet method bodies lack.
		return pyTopLevelRe.MatchString(content)
	case "java":
		// A full type declaration is the closest thing to a compilation unit.
		return javaTypeRe.MatchString(content)
	case "csharp":
		return csharpTypeRe.MatchString(content)
	case "javascript", "typescript":
		// Bias to fragment: require a top-level import or require.
		return jsEntrypointRe.MatchString(content)
	default:
		// ruby, php, scala, and anything else: bias to fragment. Their
		// complete blocks are compile-skipped anyway, so treating them as
		// fragments only changes the reported category, not the outcome.
		return false
	}
}
