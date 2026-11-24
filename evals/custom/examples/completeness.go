package examples

import "regexp"

// The completeness detectors decide whether an SDK code block is a
// self-contained compilation unit (code-complete) or an intentional snippet
// (code-fragment). Snippets dominate skill content: import blocks, method
// bodies, and elided examples. The detectors are deliberately conservative —
// when a block is ambiguous they return false so it is treated as a fragment,
// because a false "complete" wrongly fails a run while a false "fragment" only
// skips a compile. Go and Rust complete blocks are compiled, so their
// detectors follow explicit rules; the other languages have no compiler wired
// yet, and their heuristics are simple and bias to fragment.

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
	case "rust":
		// A Rust block is a compilation unit iff it names its file with a
		// "// src/<path>.rs" first line or declares fn main; see rust.go.
		return isCompleteRust(content)
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
		// Bias to fragment: require a top-level import or require, or a
		// "// <path>" first line naming the file; see javascript.go.
		return jsEntrypointRe.MatchString(content) || javascriptBlockPath(content) != ""
	default:
		// ruby, php, scala, and anything else: bias to fragment. Their
		// complete blocks would be compile-skipped anyway, so treating them
		// as fragments only changes the reported category, not the outcome.
		return false
	}
}
