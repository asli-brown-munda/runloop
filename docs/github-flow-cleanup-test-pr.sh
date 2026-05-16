#!/usr/bin/env bash
set -euo pipefail
set +x

: "${GITHUB_TOKEN:?set GITHUB_TOKEN first}"

meta="${1:-${E2E_META:-}}"
if [ -z "$meta" ] && [ -f /tmp/runloop-github-e2e-latest ]; then
  meta="$(cat /tmp/runloop-github-e2e-latest)"
fi
if [ -z "$meta" ] || [ ! -f "$meta" ]; then
  printf 'usage: GITHUB_TOKEN=... %s /path/to/meta.json\n' "$0" >&2
  exit 1
fi

command -v gh >/dev/null 2>&1 || {
  printf 'missing required command: gh\n' >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || {
  printf 'missing required command: jq\n' >&2
  exit 1
}

export GH_TOKEN="$GITHUB_TOKEN"

owner="$(jq -r '.owner' "$meta")"
repo="$(jq -r '.repo' "$meta")"
pr="$(jq -r '.prNumber' "$meta")"
branch="$(jq -r '.branch' "$meta")"

gh api "repos/$owner/$repo/pulls/$pr" -X PATCH -f state=closed >/dev/null
gh api "repos/$owner/$repo/git/refs/heads/$branch" -X DELETE >/dev/null 2>&1 || true

printf 'closed_pr=%s\n' "$pr"
printf 'deleted_branch=%s\n' "$branch"
