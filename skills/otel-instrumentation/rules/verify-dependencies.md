# Verifying instrumentation dependencies

Never write an instrumentation package name or version from memory.
Knowledge of package ecosystems goes stale in both directions: versions get invented that were never published, and packages that once existed get retired with no further releases.
A retired package still installs from the registry at its last release, so a successful install alone does not prove the instrumentation is current.

Before adding any instrumentation dependency to a manifest, verify all three against the package registry:

1. **Existence** — the package name is real.
2. **Version** — the exact version or range you reference has been published.
3. **Currency** — the package is not deprecated, yanked, or retired, and has a release compatible with the SDK version you target.

## Decision process

1. Prefer the package manager's add command without a version (`npm install <pkg>`, `go get <module>@latest`, `pip install <pkg>`, `bundle add <gem>`, `composer require <pkg>`, `dotnet add package <id>`).
   It resolves the latest published version from the registry and fails loudly when the package does not exist.
2. When you must write a manifest entry by hand, first run the lookup command from your language's section (indexed below) and copy the version it reports.
3. When the lookup shows the package is deprecated, yanked, or has no recent release, do not add it: fall back to the auto-instrumentation path or manual instrumentation per the language's SDK rule, and state why.
4. When the lookup finds no package at all, do not invent a name: check the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/) for the library, and if nothing current exists, instrument manually per [spans](./spans.md) and the SDK rule.
5. Never guess a version to complete a manifest entry.
   A wrong pin fails the build (`npm error notarget`, `go: no matching versions`), or worse, resolves to an incompatible release.

<!-- eval:bad -->
```json
{
  "dependencies": {
    "@opentelemetry/instrumentation-undici": "^0.57.0"
  }
}
```

The version above was written from memory; no `0.57.x` release of that package was ever published, and `npm install` fails with `npm error code ETARGET`.

## Language-specific verification commands

Each SDK rule carries a "Verifying dependencies" section with the concrete lookup commands for its ecosystem:

- [nodejs](./sdks/nodejs.md#verifying-dependencies) — npm
- [nextjs](./sdks/nextjs.md#verifying-dependencies) — npm
- [python](./sdks/python.md#verifying-dependencies) — pip and `opentelemetry-bootstrap`
- [go](./sdks/go.md#verifying-dependencies) — the Go module proxy
- [java](./sdks/java.md#verifying-dependencies) — Maven Central
- [scala](./sdks/scala.md#verifying-dependencies) — Maven Central via sbt/Coursier
- [dotnet](./sdks/dotnet.md#verifying-dependencies) — NuGet
- [ruby](./sdks/ruby.md#verifying-dependencies) — RubyGems
- [php](./sdks/php.md#verifying-dependencies) — Packagist

`browser` does not have dedicated guidance, as all the instrumentation is integrated in the Dash0 Web SDK.