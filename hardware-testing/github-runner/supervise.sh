#!/bin/sh
# Entry point of the `supervisor` compose service. Runs the runner as one
# fresh container per job: the runner is ephemeral and exits after a single
# job, and `docker run --rm` deletes its container, so every job starts from
# the image. See docker-compose.yaml for why.
#
# The supervisor registers each runner itself, with a just-in-time (JIT)
# configuration: a single-use registration for one ephemeral runner. The
# runner container gets only that configuration, never ACCESS_TOKEN, so job
# code cannot reach the token even as root inside the container.
set -eu

name=terrifi-github-runner-job
api="https://api.github.com/repos/${REPO_URL#https://github.com/}/actions/runners"
runner_id=

github() {
  curl --fail --silent --show-error --connect-timeout 10 --max-time 30 \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$@"
}

# An ephemeral runner removes itself after its job. Delete it anyway, in case
# it never ran one. Ignore errors, including 404 for an already removed runner.
deregister() {
  if [ -n "$runner_id" ]; then
    github -X DELETE "$api/$runner_id" >/dev/null 2>&1 || true
    runner_id=
  fi
}

# On `docker compose down` or a restart, stop the runner and exit. docker stop
# sends SIGTERM right away, which cancels any running job; the timeout only
# delays the SIGKILL if the runner does not exit.
stop() {
  docker stop --time 300 "$name" >/dev/null 2>&1 || true
  deregister
  exit 0
}
trap stop TERM INT

while true; do
  # A container left over from a supervisor crash may still be running a job.
  # Let it finish rather than killing it.
  docker wait "$name" >/dev/null 2>&1 || true
  docker rm --force "$name" >/dev/null 2>&1 || true

  # Runner names must be unique, and a removed runner's name can linger
  # briefly, so add a timestamp. Runner group 1 is the repository's Default
  # group.
  body="$(jq -n \
    --arg name "$RUNNER_NAME-$(date +%s)" \
    --arg labels "$LABELS" \
    --arg work "$RUNNER_WORKDIR" \
    '{name: $name, runner_group_id: 1, labels: ($labels | split(",")), work_folder: $work}')"
  if ! response="$(github -X POST -d "$body" "$api/generate-jitconfig")"; then
    echo "Registering a runner failed. Retrying in 60 seconds." >&2
    sleep 60
    continue
  fi
  runner_id="$(printf '%s' "$response" | jq -r .runner.id)"
  # Exported only for the docker run below, which forwards it by name so the
  # value does not appear in the supervisor's process list.
  JIT_CONFIG="$(printf '%s' "$response" | jq -r .encoded_jit_config)"
  export JIT_CONFIG

  # Background and wait, so the trap can run while a job is in progress.
  # unifi.local is the name on the controller's certificate, which the image
  # trusts. Resolve it here rather than through DNS on the HIL gateway, which
  # the tests change.
  #
  # Run the runner directly as the unprivileged `runner` user, bypassing the
  # base image's entrypoint, which registers with ACCESS_TOKEN as root. The
  # shell removes JIT_CONFIG from the environment that jobs inherit. Jobs can
  # still read the configuration from the runner's arguments, but it only
  # authenticates this runner, which GitHub removes after the job.
  docker run --rm --init --name "$name" --network "$RUNNER_NETWORK" \
    --add-host "unifi.local:$CONTROLLER_LAN_IP" \
    --user runner --env GOPROXY --env JIT_CONFIG \
    --entrypoint /bin/sh "$RUNNER_IMAGE" \
    -c 'jit="$JIT_CONFIG"; unset JIT_CONFIG; exec /actions-runner/run.sh --jitconfig "$jit"' &
  unset JIT_CONFIG
  wait $! || true
  deregister

  # Avoid a tight loop if the runner fails at startup.
  sleep 5
done
