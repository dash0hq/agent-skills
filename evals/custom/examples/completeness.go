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

// csharpTypeRe matches a C# namespace or a top-level class, interface,
// struct, record, or enum declaration at column 0.
var csharpTypeRe = regexp.MustCompile(`(?m)^(namespace|((public|internal|sealed|static|abstract|partial|readonly)\s+)*(class|interface|struct|record|enum))\s+\w`)

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
		// most snippet method bodies lack, or a "# <path>.py" first line
		// naming the file; see python.go.
		return pyTopLevelRe.MatchString(content) || pythonBlockPath(content) != ""
	case "java":
		// A Java block is a compilation unit iff it names its file with a
		// "// src/main/java/<path>.java" first line or declares a top-level
		// type at column 0; see java.go.
		return isCompleteJava(content)
	case "csharp":
		// A C# block is a compilation unit iff it names its file with a
		// "// <path>.cs" first line or declares a namespace or top-level
		// type; see csharp.go.
		return isCompleteCSharp(content)
	case "javascript", "typescript":
		// Bias to fragment: require a top-level import or require, or a
		// "// <path>" first line naming the file; see javascript.go.
		return jsEntrypointRe.MatchString(content) || javascriptBlockPath(content) != ""
	case "ruby":
		// A Ruby block is a self-contained file iff its first line is a
		// "# <path>.rb" comment or it requires a library at the top level;
		// see ruby.go.
		return isCompleteRuby(content)
	default:
		// php, scala, and anything else: bias to fragment. Their
		// complete blocks would be compile-skipped anyway, so treating them
		// as fragments only changes the reported category, not the outcome.
		return false
	}
}
