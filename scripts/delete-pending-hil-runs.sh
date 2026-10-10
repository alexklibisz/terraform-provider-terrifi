#!/usr/bin/env bash
# Delete scheduled CI - HIL runs that never executed on a runner.
# Processes only the first page (up to 100 scheduled runs) per invocation.
# Includes completed/cancelled runs; excludes PR and manual runs.
# Requires gh authentication with Actions write permission.
# Usage: ./scripts/delete-pending-hil-runs.sh [--dry-run]

set -euo pipefail

# Keep gh API responses from opening an interactive pager (including DELETE).
export GH_PAGER=cat

dry_run=false
case "${1:-}" in
  '') ;;
  --dry-run) dry_run=true ;;
  *) echo "Usage: $0 [--dry-run]" >&2; exit 1 ;;
esac
if [[ $# -gt 1 ]]; then
  echo "Usage: $0 [--dry-run]" >&2
  exit 1
fi

repo=alexklibisz/terraform-provider-terrifi
workflow=ci-hil.yaml
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

# started_at can be populated for jobs that only waited in the queue.
# Inspect all attempts for evidence of runner assignment or actual steps.
never_executed() {
  local evidence
  evidence=$(gh api --paginate "$1/jobs?filter=all&per_page=100" \
    --jq '.jobs[] | select((.runner_id // 0) != 0 or (.runner_name // "") != "" or (.steps | length) > 0 or .status == "in_progress") | .id') || return 2
  [[ -z "$evidence" ]]
}

failed=0
matched=0
process_run() {
  local run_id="$1" endpoint evidence result metadata event status attempt
  endpoint="repos/$repo/actions/runs/$run_id"
  echo "Checking scheduled run $run_id for executed jobs..."
  if never_executed "$endpoint"; then
    :
  else
    result=$?
    if [[ $result == 2 ]]; then
      failed=1
    fi
    return 0
  fi

  # Recheck state because a queued run may have started since the snapshot.
  metadata=$(gh api "$endpoint" --jq '[.event, .status] | @tsv')
  IFS=$'\t' read -r event status <<< "$metadata"
  if [[ "$event" != schedule || "$status" == in_progress ]]; then
    echo "Skipping run $run_id: event=$event, status=$status."
    return 0
  fi
  matched=$((matched + 1))
  if "$dry_run"; then
    echo "Would delete never-executed scheduled run: https://github.com/$repo/actions/runs/$run_id (status=$status)"
    return 0
  fi

  if [[ "$status" != completed ]]; then
    echo "Cancelling scheduled run $run_id..."
    if ! gh api --method POST "$endpoint/cancel"; then
      failed=1
      return 0
    fi
    echo "Waiting for run $run_id to finish cancelling..."
    for ((attempt = 0; attempt < 60; attempt++)); do
      status=$(gh api "$endpoint" --jq '.status')
      if [[ "$status" == completed ]]; then
        break
      fi
      sleep 2
    done
    if [[ "$status" != completed ]]; then
      echo "Timed out waiting for run $run_id; leaving it undeleted." >&2
      failed=1
      return 0
    fi
  fi

  # Preserve a run if a runner picked it up during cancellation.
  if never_executed "$endpoint"; then
    echo "Deleting never-executed scheduled run $run_id..."
    if ! gh api --method DELETE "$endpoint"; then
      failed=1
    fi
  else
    result=$?
    echo "Leaving run $run_id undeleted: execution evidence or job lookup failure."
    if [[ $result == 2 ]]; then
      failed=1
    fi
  fi
}

# Snapshot one page, then process it immediately. Re-running fetches the new
# first page after deleted runs have been removed.
echo "Fetching first page of scheduled $workflow runs in $repo..."
gh api "repos/$repo/actions/workflows/$workflow/runs?per_page=100&page=1&event=schedule" \
  --jq '.workflow_runs[] | select(.event == "schedule") | .id' \
  > "$work_dir/runs"

while IFS= read -r run_id; do
  process_run "$run_id"
done < "$work_dir/runs"

echo "Found $matched never-executed scheduled runs."
exit "$failed"
