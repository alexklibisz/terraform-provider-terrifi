#!/bin/sh
# Entry point of the `supervisor` compose service. Runs the runner as one
# fresh container per job: the runner is ephemeral and exits after a single
# job, and `docker run --rm` deletes its container, so every job starts from
# the image. See docker-compose.yaml for why.
set -eu

name=terrifi-github-runner-job

# On `docker compose down` or a restart, stop the runner and exit. docker stop
# sends SIGTERM right away, which cancels any running job; the timeout only
# delays the SIGKILL if the runner does not exit.
stop() {
  docker stop --time 300 "$name" >/dev/null 2>&1 || true
  exit 0
}
trap stop TERM INT

while true; do
  # A container left over from a supervisor crash may still be running a job.
  # Let it finish rather than killing it.
  docker wait "$name" >/dev/null 2>&1 || true
  docker rm --force "$name" >/dev/null 2>&1 || true

  # Background and wait, so the trap can run while a job is in progress.
  docker run --rm --name "$name" --network "$RUNNER_NETWORK" \
    --env GOPROXY --env REPO_URL --env ACCESS_TOKEN --env RUNNER_NAME --env RUNNER_WORKDIR \
    --env LABELS --env RUNNER_GROUP --env EPHEMERAL --env RUN_AS_ROOT \
    "$RUNNER_IMAGE" &
  wait $! || true

  # Avoid a tight loop if the runner fails at startup, e.g. a bad token.
  sleep 5
done
