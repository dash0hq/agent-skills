# Changelog

## v1.2.5 (2026-05-04)

### Changed

- update plugin manifests for v1.2.5
- split patterns out from OTTL skills


## v1.2.4 (2026-05-04)

### Fixed

- replace hardcoded opentelemetry-javaagent version with placeholder
- tighten SDK install scripts and k8s note
- switch scala sbt example to Maven Central via sbt-javaagent
- harden custom-namespace and CLIENT/SERVER URL rules
- set OTEL_*_EXPORTER=otlp explicitly in k8s pod spec
- pin SDK install URLs to versioned releases

### Changed

- update plugin manifests for v1.2.4
- remove zipkin from supported OTel exporter documentation for Kubernetes
- clarify OTEL_*_EXPORTER spec semantics in k8s pod spec
- add tessl badge
- update installation instructions and credit Tessl for various improvements


## v1.2.3 (2026-05-01)

### Changed

- update plugin manifests for v1.2.3
- move installation instructions to INSTALL.md and add tessl


## v1.2.2 (2026-05-01)

### Fixed

- tile validation

### Changed

- update plugin manifests for v1.2.2
- update tessl tile version on release


## v1.2.1 (2026-05-01)

### Fixed

- fix broken links due to refactoring
- dangling reference

### Changed

- update plugin manifests for v1.2.1
- add tessl linting for broken links in skills
- publish to tessl on release
- onboard skills to tessl.io


## v1.2.0 (2026-05-01)

### Added

- add plugin manifests for simpler installation (#8)

### Changed

- update plugin manifests for v1.2.0
- extract redaction to rule file, split patterns file
- move advanced patterns to rule file
- avoid repetition in YAML snippets
- trim explanatory sentences
- move component reference to rule file
- improve SKILL.md
- Reference otel-instrumentation for span naming, improve SKILL.md
- add validation instructions
- extract function reference to separate file, add missing functions
- use flat metadata
- add quick-start for the otel-collector skill
- fix install instructions for Claude Code
- bump actions/checkout from 4 to 6 (#7)
- add license
- add Dependabot configuration for GitHub Actions (#6)


## v1.1.0 (2026-03-30)

### Added

- add OCB, more info on http.route recording, and OTTL support


## v1.0.4 (2026-03-20)

### Added

- document in collector skill how to improve Dash0 Operator in GitOps setups

### Changed

- fix changelog update


## v1.0.3 (2026-03-20)

### Added

- add guidance about information redaction and related best practices

### Changed

- mention the ottl skill in the CLAUDE.md example

## v1.0.2 (2026-03-18)

### Fixed

- bug in the sampling YAML and Node.js export protocols

## v1.0.1 (2026-03-13)

### Added

- improve guidance for resource attributes