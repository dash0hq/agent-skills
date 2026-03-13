# Releasing a new version

This repository uses [GitHub releases](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) with [semantic versioning](https://semver.org/).
Consumers install skills via `npx skills add dash0hq/agent-skills`, which resolves the latest release from the repository.

## Version numbering

Follow semantic versioning (`MAJOR.MINOR.PATCH`):

- **MAJOR** — breaking changes to skill structure or filenames that would cause existing agent integrations to fail (e.g., renaming a `SKILL.md`, removing a rule file, changing the directory layout).
- **MINOR** — new skills, new rule files, or significant new guidance within existing rules.
- **PATCH** — typo fixes, wording improvements, corrections to existing guidance, and small additions that do not change the skill surface.

## Pre-release checklist

1. Ensure all changes are merged to `main`.
2. Review the diff since the last release:

   ```bash
   git log --oneline $(git describe --tags --abbrev=0 2>/dev/null || git rev-list --max-parents=0 HEAD)..HEAD
   ```

3. Verify that `README.md` reflects the current set of skills and their descriptions.
4. Confirm that every skill directory contains a valid `SKILL.md` and a `rules/` directory.

## Creating the release

1. Go to [Actions > Release](https://github.com/dash0hq/agent-skills/actions/workflows/release.yml).
2. Click **Run workflow**.
3. Select the bump type from the dropdown: `major`, `minor`, or `patch`.
4. Click **Run workflow** to start the release.
5. Review the generated release notes on the [releases page](https://github.com/dash0hq/agent-skills/releases) and edit them if needed to group changes by skill and clarify impact.

## What the workflow does

1. **Validate skills** — checks that every directory under `skills/` contains a `SKILL.md` and a `rules/` directory.
2. **Compute next version** — reads the latest tag and increments the selected component (e.g., `v1.2.3` with `minor` becomes `v1.3.0`).
   If no tag exists yet, the base version is `v0.0.0`.
3. **Generate changelog** — collects commits since the last tag, groups them by conventional commit type (`feat` → Added, `fix` → Fixed, everything else → Changed), and prepends a new entry to `CHANGELOG.md`.
4. **Commit changelog** — commits the updated `CHANGELOG.md` to `main`.
5. **Create tag** — creates and pushes an annotated `vMAJOR.MINOR.PATCH` tag on the changelog commit.
6. **Create GitHub release** — publishes a release with auto-generated notes from the commit history since the previous tag.

## Post-release verification

1. Confirm the release is visible on the [releases page](https://github.com/dash0hq/agent-skills/releases).
2. Verify installation resolves the new version:

   ```bash
   npx skills add dash0hq/agent-skills
   ```
