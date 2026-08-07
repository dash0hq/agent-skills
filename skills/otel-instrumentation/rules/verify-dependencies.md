# Verifying instrumentation dependencies

Never write an instrumentation package name or version from memory.
Knowledge of package ecosystems goes stale in both directions: versions get invented that were never published, and packages that once existed get retired with no further releases.
A retired package still installs from the registry at its last release, so a successful install alone does not prove the instrumentation is current.

Before adding any instrumentation dependency to a manifest, verify all four against the package registry:

1. **Existence** — the package name is real.
2. **Version** — the exact version or range you reference has been published.
3. **Currency** — the package is not deprecated, yanked, or retired, and has a release compatible with the SDK version you target.
4. **Compatibility** — the version's runtime or toolchain requirement is satisfied by the project: the newest published release is not the newest usable one when it demands a newer language toolchain, runtime, or framework than the project builds and runs on.

## Decision process

1. Prefer the package manager's add command without a version (`npm install <pkg>`, `go get <module>@latest`, `pip install <pkg>`, `bundle add <gem>`, `composer require <pkg>`, `dotnet add package <id>`).
   It resolves the latest published version from the registry and fails loudly when the package does not exist.
2. When you must write a manifest entry by hand, first run the lookup command from your language's section (indexed below) and copy the version it reports.
3. Check the version's runtime or toolchain requirement against the project before pinning it: the project's manifest (for example the `go` directive in `go.mod`, `engines` in `package.json`) and its build and runtime environments (base images in Dockerfiles, CI toolchains).
   When the newest release requires a newer toolchain than the project has, pin the newest release that satisfies the project's toolchain instead — or upgrade the toolchain deliberately, as its own visible change (manifest directive, base images, CI), never as a side effect of a dependency bump.
4. After any manifest edit, bring the lockfile in step per [Keeping the lockfile in step](#keeping-the-lockfile-in-step): lockfile entries cannot be hand-written, and a stale lockfile fails strict installs and Go builds.
5. When the lookup shows the package is deprecated, yanked, or has no recent release, do not add it: fall back to the auto-instrumentation path or manual instrumentation per the language's SDK rule, and state why.
   When this means removing a dependency that is already in the manifest, follow [Dropping an existing dependency](#dropping-an-existing-dependency).
6. When the lookup finds no package at all, do not invent a name: check the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/) for the library, and if nothing current exists, instrument manually per [spans](./spans.md) and the SDK rule.
7. Never guess a version to complete a manifest entry.
   A wrong pin fails the build (`npm error notarget`, `go: no matching versions`, `go: module ... requires go >= ...`), or worse, resolves to an incompatible release.

<!-- eval:bad -->
```json
{
  "dependencies": {
    "@opentelemetry/instrumentation-undici": "^0.57.0"
  }
}
```

The version above was written from memory; no `0.57.x` release of that package was ever published, and `npm install` fails with `npm error code ETARGET`.

## Keeping the lockfile in step

A dependency manifest and its lockfile move together: `go.mod` with `go.sum`, `package.json` with `package-lock.json`, `Gemfile` with `Gemfile.lock`, and `composer.json` with `composer.lock`.
The lockfile records resolver output and content hashes that only the package manager can compute, so a lockfile entry cannot be hand-written.
Editing the manifest without regenerating the lockfile breaks the next build: strict installs refuse the mismatch by design (`npm ci`, frozen Bundler installs, Composer installing from a lockfile that no longer satisfies `composer.json`), and Go refuses to compile (`missing go.sum entry for module providing package ...`).

Regenerate the lockfile with the ecosystem's lock-refreshing command (`go mod tidy`, `npm install`, `bundle lock`, `composer update <pkg>`) in the same change as every manifest edit, and ship both files together.
When the edit happens somewhere the toolchain cannot run — the change is applied through a web interface or a review suggestion, made by a CI bot or dependency tooling, or the project only builds inside a container — regenerate the lockfile where the toolchain does run.
For container builds, that place is the builder stage itself: make it self-sufficient by regenerating the lockfile before the build or install step, using the command from the language's "Verifying dependencies" section (indexed below).

## Dropping an existing dependency

Removing an instrumentation dependency makes the telemetry it produced disappear, and dashboards and alerts may be built on that telemetry.
When an existing instrumentation dependency cannot survive a change — it is retired and blocks an upgrade, or its pins conflict with the rest of the dependency set — do not drop it silently.

Ask the user for confirmation before removing it, naming:

1. The package, and why it cannot be kept.
2. Exactly which telemetry disappears: the library whose spans, metrics, or logs stop being produced.
3. The replacement, if any — native client telemetry or manual instrumentation per the language's SDK rule — and any span names or attributes that change with it, since renamed telemetry breaks dashboards and alerts just like removed telemetry.

When you have no way to ask — running non-interactively or in a pipeline — proceed only if the task cannot complete otherwise, and report the removal and its telemetry impact prominently in your summary.

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