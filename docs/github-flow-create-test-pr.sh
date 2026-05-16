#!/usr/bin/env bash
set -euo pipefail
set +x

: "${GITHUB_TOKEN:?set GITHUB_TOKEN first}"
: "${OWNER:?set OWNER first, for example OWNER=asli-brown-munda}"
: "${REPO:?set REPO first, for example REPO=runloop}"

require() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  }
}

require gh
require jq

export GH_TOKEN="$GITHUB_TOKEN"

stamp="$(date -u +%Y%m%d%H%M%S)"
root="/tmp/runloop-github-e2e-$stamp"
mkdir -p "$root"
meta="$root/meta.json"
state="$root/state.json"
printf '{"stage":"start"}\n' > "$state"

record() {
  jq "$@" "$state" > "$state.tmp"
  mv "$state.tmp" "$state"
}

stage() {
  record --arg stage "$1" '.stage=$stage'
}

cleanup_partial() {
  status=$?
  if [ "$status" -ne 0 ]; then
    printf 'failed at stage: %s\n' "$(jq -r '.stage // "unknown"' "$state" 2>/dev/null || printf unknown)" >&2
    pr="$(jq -r '.prNumber // empty' "$state" 2>/dev/null || true)"
    branch="$(jq -r '.branch // empty' "$state" 2>/dev/null || true)"
    if [ -n "$pr" ]; then
      gh api "repos/$OWNER/$REPO/pulls/$pr" -X PATCH -f state=closed >/dev/null 2>&1 || true
    fi
    if [ -n "$branch" ]; then
      gh api "repos/$OWNER/$REPO/git/refs/heads/$branch" -X DELETE >/dev/null 2>&1 || true
    fi
  fi
  exit "$status"
}

trap cleanup_partial EXIT

stage viewer
viewer="$(gh api graphql -f query='query { viewer { login } }' --jq '.data.viewer.login')"

stage repo
repo_json="$(
  gh api graphql \
    -f query='query($owner:String!, $name:String!) { repository(owner:$owner, name:$name) { nameWithOwner viewerPermission defaultBranchRef { name target { oid } } } }' \
    -F owner="$OWNER" \
    -F name="$REPO"
)"
name_with_owner="$(jq -r '.data.repository.nameWithOwner' <<<"$repo_json")"
permission="$(jq -r '.data.repository.viewerPermission' <<<"$repo_json")"
base_branch="$(jq -r '.data.repository.defaultBranchRef.name' <<<"$repo_json")"
base_oid="$(jq -r '.data.repository.defaultBranchRef.target.oid' <<<"$repo_json")"

branch="runloop-e2e-github-poll-$stamp"
path=".runloop-e2e/github-poll-$stamp.md"

stage blob
content="# Runloop GitHub poll E2E $stamp

Temporary file created by a Runloop GitHub poll end-to-end smoke test.

Review target line for Runloop poll.
"
blob_sha="$(gh api "repos/$OWNER/$REPO/git/blobs" -X POST -f content="$content" -f encoding='utf-8' --jq '.sha')"

stage tree
tree_payload="$(
  jq -n \
    --arg base "$base_oid" \
    --arg path "$path" \
    --arg sha "$blob_sha" \
    '{base_tree:$base, tree:[{path:$path, mode:"100644", type:"blob", sha:$sha}]}'
)"
tree_sha="$(gh api "repos/$OWNER/$REPO/git/trees" -X POST --input - <<<"$tree_payload" --jq '.sha')"

stage commit
commit_payload="$(
  jq -n \
    --arg msg "Runloop GitHub poll E2E $stamp" \
    --arg tree "$tree_sha" \
    --arg parent "$base_oid" \
    '{message:$msg, tree:$tree, parents:[$parent]}'
)"
head_sha="$(gh api "repos/$OWNER/$REPO/git/commits" -X POST --input - <<<"$commit_payload" --jq '.sha')"

stage ref
ref_payload="$(jq -n --arg ref "refs/heads/$branch" --arg sha "$head_sha" '{ref:$ref, sha:$sha}')"
gh api "repos/$OWNER/$REPO/git/refs" -X POST --input - <<<"$ref_payload" >/dev/null
record --arg branch "$branch" '.branch=$branch'

stage pr
pr_body="Temporary PR for Runloop GitHub source polling E2E. Created $stamp UTC and safe to close after verification."
pr_json="$(
  gh api "repos/$OWNER/$REPO/pulls" \
    -X POST \
    -f title="Runloop GitHub poll E2E $stamp" \
    -f head="$branch" \
    -f base="$base_branch" \
    -f body="$pr_body"
)"
pr_number="$(jq -r '.number' <<<"$pr_json")"
pr_url="$(jq -r '.html_url' <<<"$pr_json")"
record --argjson prNumber "$pr_number" '.prNumber=$prNumber'

stage assign
gh api "repos/$OWNER/$REPO/issues/$pr_number/assignees" -X POST -f assignees[]="$viewer" >/dev/null

stage review_comment
comment_body="Runloop E2E unresolved review thread marker $stamp. Claude should only report that this smoke-test thread was observed."
comment_json="$(
  gh api "repos/$OWNER/$REPO/pulls/$pr_number/comments" \
    -X POST \
    -f body="$comment_body" \
    -f commit_id="$head_sha" \
    -f path="$path" \
    -F line=1 \
    -f side='RIGHT'
)"
comment_url="$(jq -r '.html_url' <<<"$comment_json")"

stage verify_thread
sleep 2
thread_json="$(
  gh api graphql \
    -f query='query($owner:String!, $name:String!, $number:Int!) { repository(owner:$owner, name:$name) { pullRequest(number:$number) { reviewThreads(first: 20) { nodes { id isResolved comments(first: 5) { nodes { body url } } } } } } }' \
    -F owner="$OWNER" \
    -F name="$REPO" \
    -F number="$pr_number"
)"
unresolved_count="$(jq '[.data.repository.pullRequest.reviewThreads.nodes[]? | select(.isResolved == false)] | length' <<<"$thread_json")"
if [ "$unresolved_count" -lt 1 ]; then
  printf 'no unresolved review thread visible after comment creation\n' >&2
  exit 1
fi

jq -n \
  --arg root "$root" \
  --arg owner "$OWNER" \
  --arg repo "$REPO" \
  --arg nameWithOwner "$name_with_owner" \
  --arg viewer "$viewer" \
  --arg permission "$permission" \
  --arg baseBranch "$base_branch" \
  --arg baseOid "$base_oid" \
  --arg branch "$branch" \
  --arg headSha "$head_sha" \
  --arg path "$path" \
  --arg prNumber "$pr_number" \
  --arg prUrl "$pr_url" \
  --arg reviewCommentUrl "$comment_url" \
  --arg unresolvedCount "$unresolved_count" \
  --arg stamp "$stamp" \
  '{
    root:$root,
    owner:$owner,
    repo:$repo,
    nameWithOwner:$nameWithOwner,
    viewer:$viewer,
    permission:$permission,
    baseBranch:$baseBranch,
    baseOid:$baseOid,
    branch:$branch,
    headSha:$headSha,
    path:$path,
    prNumber:($prNumber|tonumber),
    prUrl:$prUrl,
    reviewCommentUrl:$reviewCommentUrl,
    unresolvedCount:($unresolvedCount|tonumber),
    stamp:$stamp
  }' > "$meta"

printf '%s\n' "$meta" > /tmp/runloop-github-e2e-latest
trap - EXIT

jq '{meta:"'"$meta"'", viewer, permission, nameWithOwner, branch, headSha, path, prNumber, prUrl, reviewCommentUrl, unresolvedCount}' "$meta"
