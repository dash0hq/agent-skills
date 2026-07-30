# Releasing a new version

This repository uses [GitHub releases](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) with [semantic versioning](https://semver.org/).
Consumers install skills via `npx skills add dash0hq/agent-skills`, which resolves the latest release from the repository.

## Version numbering

Follow semantic versioning (`MAJOR.MINOR.PATCH`):

- **MAJOR** — breaking changes to skill structure or filenames that would cause existing agent integrations to fail (e.g., renaming a `SKILL.md`, removing a rule file, changing the directory layout).
- **MINOR** — new skills, new rule files, or significant new guidance within existing rules.
- **PATCH** — typo fixes, wording improvements, corrections to existing guidance, and small additions that do not change the skill surface.

## Published artifact contents

The eval harness (`evals/custom/`), the Tessl eval scenarios (`evals/tessl/`), and the repository docs (`docs/`) are maintainer-facing and must not ship in published artifacts.
The whole `evals/` tree and `docs/` are therefore excluded.

The Tessl tile excludes them through [`.tesslignore`](./.tesslignore) at the repository root, which `tessl plugin publish` reads with gitignore-style patterns; [`.tileignore`](./.tileignore) carries the same patterns for tessl CLI versions that predate the tile-to-plugin rename.
Verify the exclusion before releasing:

```bash
tessl plugin publish . --dry-run --verbose
```

The `--verbose` file listing must contain no paths under `evals/` or `docs/`; if it does, fix the ignore files before running the release workflow.

The npm package ([`@dash0/agent-skills`](https://www.npmjs.com/package/@dash0/agent-skills)) excludes them through the `files` allowlist in `package.json`, which ships only `skills/` (plus `README.md`, `LICENSE`, and `package.json`).
Verify with:

```bash
npm pack --dry-run
```

The release workflow re-checks this and publishes with `npm publish --provenance`, authenticated with the `DASH0_NPMJS_PUBLISH_TOKEN` organization secret.
(Not [OIDC trusted publishing](https://docs.npmjs.com/trusted-publishers): npm validates the calling workflow's name and allows one trusted publisher per package, which cannot cover both the release path and the manual-dispatch path.)
The workflow also bumps the `version` field in `package.json` (bare semver, no `v` prefix) alongside the plugin manifests.

The Claude Code plugin, the Cursor plugin, and the Gemini CLI extension install by cloning this repository.
Neither `plugin.json` nor `gemini-extension.json` supports an include or exclude field, and no ignore-file mechanism exists for those installers (verified against the [Claude Code plugin reference](https://code.claude.com/docs/en/plugins-reference) and the [Gemini CLI extension reference](https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md) as of 2026-07).
Those installs therefore contain `evals/` and `docs/`; the content is inert for consumers, and removing it would require a separate distribution repository or release archives instead of git clones.

### Scoring the Tessl scenarios at publish

The release workflow passes `--eval-scenarios evals/tessl` to the publish command, so the authored scenarios in `evals/tessl/` are scored on the registry at publish time.
The flag reads the scenarios from the working tree, which carries them regardless of the `.tesslignore` exclusion, so they contribute to the registry score without shipping inside the packaged tile.

## Pre-release checklist

1. Ensure all changes are merged to `main`.
2. Review the diff since the last release:

   ```bash
   git log --oneline $(git describe --tags --abbrev=0 2>/dev/null || git rev-list --max-parents=0 HEAD)..HEAD
   ```

3. Verify that `README.md` reflects the current set of skills and their descriptions.
4. Confirm that every skill directory contains a valid `SKILL.md`.
5. Verify the Tessl tile contents with `tessl plugin publish . --dry-run --verbose` and confirm the listing contains no `evals/` or `docs/` paths.

## Creating the release

1. Go to [Actions > Release](https://github.com/dash0hq/agent-skills/actions/workflows/release.yml).
2. Click **Run workflow**.
3. Select the bump type from the dropdown: `major`, `minor`, or `patch`.
4. Click **Run workflow** to start the release.
5. Review the generated release notes on the [releases page](https://github.com/dash0hq/agent-skills/releases) and edit them if needed to group changes by skill and clarify impact.

## What the workflow does

1. **Validate skills** — checks that every directory under `skills/` contains a `SKILL.md`.
2. **Compute next version** — reads the latest tag and increments the selected component (e.g., `v1.2.3` with `minor` becomes `v1.3.0`).
   If no tag exists yet, the base version is `v0.0.0`.
3. **Generate changelog** — collects commits since the last tag, groups them by conventional commit type (`feat` → Added, `fix` → Fixed, everything else → Changed), and prepends a new entry to `CHANGELOG.md`.
4. **Commit changelog** — commits the updated `CHANGELOG.md` to `main`.
5. **Create tag** — creates and pushes an annotated `vMAJOR.MINOR.PATCH` tag on the changelog commit.
6. **Create GitHub release** — publishes a release with auto-generated notes from the commit history since the previous tag.
7. **Publish to npm** — publishes `@dash0/agent-skills` (the `skills/` trees only) to npmjs.com with provenance, authenticated with the `DASH0_NPMJS_PUBLISH_TOKEN` organization secret. This runs as a separate job that calls [`publish-npm.yml`](./.github/workflows/publish-npm.yml) with the new tag; the same workflow can also be [dispatched manually](https://github.com/dash0hq/agent-skills/actions/workflows/publish-npm.yml) with any existing tag — to retrofit a release cut before npm publishing existed, or to retry a failed publish without re-releasing.

## Post-release verification

1. Confirm the release is visible on the [releases page](https://github.com/dash0hq/agent-skills/releases).
2. Verify installation resolves the new version:

   ```bash
   npx skills add dash0hq/agent-skills
   ```

3. Verify the npm package:

   ```bash
   npm view @dash0/agent-skills version
   ```
