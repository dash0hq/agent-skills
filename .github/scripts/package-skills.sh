#!/usr/bin/env bash
#
# Builds one <skill>.zip and one <skill>.skill archive per directory under
# skills/, plus a dash0-agent-skills.zip bundle of every skill, and verifies
# their layout.
#
# A .skill file is a zip archive with a different extension: the format Claude
# imports directly. Both per-skill archives hold a single top-level folder named
# after the skill, with SKILL.md directly inside it; the bundle holds every
# skill folder side by side. Only skills/ is packaged, so evals/ and docs/ are
# excluded by construction.
#
# Usage:
#   package-skills.sh <out-dir>
#
# The archives are reproducible: they contain only files tracked by git, in
# sorted order, without directory entries or extra attributes, and every file
# carries the HEAD commit's timestamp. Building the same commit twice yields
# byte-identical archives, so rebuilding the assets of an existing release does
# not change their checksums.
#
# The release workflow calls this before creating the GitHub release, and
# package-skills.yml calls it to retrofit an existing tag; run it locally from
# anywhere inside the repository.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <out-dir>" >&2
  exit 2
fi

BUNDLE_NAME="dash0-agent-skills"

mkdir -p "$1"
out_dir=$(cd "$1" && pwd)

cd "$(git rev-parse --show-toplevel)"

# zip stores DOS timestamps in local time; pin the zone so the bytes do not
# depend on the machine that builds them.
export TZ=UTC
mtime=$(git log -1 --format=%cd --date=format-local:%Y%m%d%H%M.%S HEAD)

# GitHub Actions renders ::error:: annotations on the run; plain runs print the
# same message to stderr.
fail() {
  if [ -n "${GITHUB_ACTIONS:-}" ]; then
    echo "::error::$1"
  else
    echo "error: $1" >&2
  fi
  exit 1
}

staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT

for skill_dir in skills/*/; do
  skill_name=$(basename "$skill_dir")
  if [ ! -f "skills/${skill_name}/SKILL.md" ]; then
    fail "Skill '${skill_name}' is missing SKILL.md"
  fi

  # Stage a copy so the working tree's timestamps stay untouched.
  (cd skills && git ls-files -z -- "${skill_name}" | xargs -0 tar -cf - | tar -xf - -C "$staging")
  find "${staging}/${skill_name}" -type f -exec touch -t "$mtime" {} +

  rm -f "${out_dir}/${skill_name}.zip"
  (cd "$staging" && find "$skill_name" -type f | LC_ALL=C sort | zip -q -X -D -@ "${out_dir}/${skill_name}.zip")
  cp "${out_dir}/${skill_name}.zip" "${out_dir}/${skill_name}.skill"

  for archive in "${out_dir}/${skill_name}.zip" "${out_dir}/${skill_name}.skill"; do
    # Capture the listing first: grep -q exits on the first match, and under
    # pipefail the resulting SIGPIPE in unzip would fail the check at random.
    entries=$(unzip -Z1 "$archive")
    if ! grep -qx "${skill_name}/SKILL.md" <<<"$entries"; then
      fail "${archive} does not contain ${skill_name}/SKILL.md"
    fi
    if grep -qv "^${skill_name}/" <<<"$entries"; then
      fail "${archive} contains entries outside ${skill_name}/"
    fi
  done
done

# The bundle holds every skill folder side by side, for unpacking into a skills
# directory in one step. Claude refuses it as an upload (it requires exactly one
# top-level folder and one SKILL.md), so it ships as .zip only.
bundle="${out_dir}/${BUNDLE_NAME}.zip"
rm -f "$bundle"
(cd "$staging" && find . -type f | sed 's|^\./||' | LC_ALL=C sort | zip -q -X -D -@ "$bundle")
entries=$(unzip -Z1 "$bundle")
for skill_dir in skills/*/; do
  skill_name=$(basename "$skill_dir")
  if ! grep -qx "${skill_name}/SKILL.md" <<<"$entries"; then
    fail "${bundle} does not contain ${skill_name}/SKILL.md"
  fi
done
if [ "$(grep -c '/SKILL.md$' <<<"$entries")" -ne "$(find skills -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l)" ]; then
  fail "${bundle} contains SKILL.md files that are not directly inside a skill folder"
fi

ls -l "$out_dir"
echo "Archive layout verified."
