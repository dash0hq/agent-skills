#!/usr/bin/env bash
#
# Single source of truth for the manifests that carry the release version.
#
# The repository ships the same plugin under several manifest formats. The host
# manifests carry the v-prefixed release tag; package.json (npm requires bare
# semver) and plugin.json (the Agent Plugins manifest, whose format recommends
# semver) carry the same version without the prefix.
#
# Usage:
#   manifest-versions.sh check          # fail if the manifests disagree
#   manifest-versions.sh set <version>  # rewrite every manifest to <version>
#   manifest-versions.sh paths          # list every manifest, one per line
#
# The release workflow calls `set`, the pull-request workflow calls `check`;
# run either locally from anywhere inside the repository.

set -euo pipefail

V_PREFIXED=(
  ".claude-plugin/plugin.json"
  ".cursor-plugin/plugin.json"
  "gemini-extension.json"
  ".tessl-plugin/plugin.json"
)

BARE=(
  "package.json"
  "plugin.json"
)

SEMVER='^[0-9]+\.[0-9]+\.[0-9]+$'

cd "$(git rev-parse --show-toplevel)"

# GitHub Actions renders ::error:: annotations on the pull request; plain runs
# print the same message to stderr.
fail() {
  local file="$1" message="$2"
  if [ -n "${GITHUB_ACTIONS:-}" ]; then
    echo "::error file=${file}::${message}"
  else
    echo "${file}: ${message}" >&2
  fi
}

write_version() {
  local manifest="$1" version="$2"
  jq ".version = \"${version}\"" "${manifest}" > "${manifest}.tmp"
  mv "${manifest}.tmp" "${manifest}"
}

cmd_paths() {
  printf '%s\n' "${V_PREFIXED[@]}" "${BARE[@]}"
}

cmd_set() {
  local version="${1#v}"

  if ! echo "${version}" | grep -qE "${SEMVER}"; then
    echo "not a semver version: ${1}" >&2
    return 1
  fi

  local manifest
  for manifest in "${V_PREFIXED[@]}"; do
    write_version "${manifest}" "v${version}"
  done
  for manifest in "${BARE[@]}"; do
    write_version "${manifest}" "${version}"
  done

  echo "Set every manifest to v${version}."
}

cmd_check() {
  local reference="${V_PREFIXED[0]}"
  local expected
  expected=$(jq -r ".version" "${reference}")

  if ! echo "${expected#v}" | grep -qE "${SEMVER}" || [ "${expected#v}" = "${expected}" ]; then
    fail "${reference}" "version \"${expected}\" is not a v-prefixed semver"
    return 1
  fi

  local exit_code=0 manifest actual
  for manifest in "${V_PREFIXED[@]}"; do
    actual=$(jq -r ".version" "${manifest}")
    if [ "${actual}" != "${expected}" ]; then
      fail "${manifest}" "version is \"${actual}\", expected \"${expected}\""
      exit_code=1
    fi
  done
  for manifest in "${BARE[@]}"; do
    actual=$(jq -r ".version" "${manifest}")
    if [ "${actual}" != "${expected#v}" ]; then
      fail "${manifest}" "version is \"${actual}\", expected \"${expected#v}\""
      exit_code=1
    fi
  done

  if [ "${exit_code}" -eq 0 ]; then
    echo "All manifests are at ${expected}."
  fi

  return "${exit_code}"
}

case "${1:-}" in
  check) cmd_check ;;
  set)   cmd_set "${2:?usage: manifest-versions.sh set <version>}" ;;
  paths) cmd_paths ;;
  *)
    echo "usage: manifest-versions.sh check|set <version>|paths" >&2
    exit 2
    ;;
esac
