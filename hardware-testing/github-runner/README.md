# GitHub Actions Self-Hosted Runner (Docker)

This directory contains a Docker Compose setup for running a GitHub Actions self-hosted runner. The runner is used by the `CI - HIL` workflow to run acceptance tests against real UniFi hardware.

## Prerequisites

A server with Docker, Docker Compose, and rsync installed. The runner needs access to the UniFi controller on the local network.

## Creating a GitHub Personal Access Token

The runner authenticates with a Personal Access Token (classic), which it uses to automatically mint short-lived registration tokens on each startup.

1. Go to https://github.com/settings/tokens
2. Click **Generate new token** > **Generate new token (classic)**
3. Give it a descriptive name (e.g., `terrifi-gh-runner`)
4. Set an expiration (or no expiration for a long-lived runner)
5. Select the **`repo`** scope (full control of private repositories)
6. Click **Generate token** and copy the value

## Setup

1. Set up SSH access with an alias `terrifi-gh-runner`, i.e., you should be able to run `ssh terrifi-gh-runner` from your host.
2. Rsync the Docker Compose files and `run.sh` to the server: `./rsync.sh`
3. SSH into the server.
4. Copy the env file: `cp .env.example .env` and fill in `REPO_URL` and `ACCESS_TOKEN`.
5. Start the runner: `cd /home/terrifi/docker-compose && ./run.sh`
6. Confirm the runner appears at: https://github.com/your-org/terrifi/settings/actions/runners

## Updating the Runner

Run `./rsync.sh` from this directory to sync the files, then run `./run.sh` on
the server to rebuild with the latest `ubuntu-noble` base image and start the
updated container. The build uses
`--pull` because Docker otherwise may reuse a cached base image even with
`--build`.

To update directly on the server:

```sh
cd /home/terrifi/docker-compose
./run.sh
```

## Verifying the Runner in GitHub

1. Go to your repository on GitHub
2. Navigate to **Settings** > **Actions** > **Runners**
3. The runner should appear with a green "Idle" status
4. The labels `self-hosted` and `terrifi-hardware-test` should be listed

## Troubleshooting

### Note: Idle Runner Not Picking Up Jobs

On October 10, 2026, the HIL job for PR #185 stayed queued with "Waiting for a
runner to pick up this job," while GitHub showed `terrifi-hardware-runner` as
online and idle. We checked that the job's requested labels (`self-hosted` and
`terrifi-hardware-test`) matched the runner's labels. The workflow's repository
guard also evaluated to true, so neither labels nor the guard explained the wait.

The useful clue was in the container logs:

```sh
cd /home/terrifi/docker-compose
docker compose logs --tail=150 runner
```

The runner reported version `2.332.0`, downloaded an update to `2.337.0`, and
restarted after completing the update process. After restarting, it reported
`2.332.0` again. An online/idle status and "Listening for Jobs" therefore did
not establish that the runner was ready to accept jobs. GitHub can stop assigning
jobs to outdated runners; see the [runner update requirements](https://docs.github.com/en/actions/reference/runners/self-hosted-runners).

Our build command used `docker compose up -d --build`, which could reuse the
cached `myoung34/github-runner:ubuntu-noble` base image. We changed `run.sh` to
run `docker compose build --pull runner` before `docker compose up -d runner`.
The `ubuntu-noble` tag already tracks the latest runner build for Ubuntu 24.04;
refreshing the cached base image was the missing step. After rebuilding, the
runner picked up the PR job again.

## Architecture

- **Ubuntu Noble (24.04)** base image via [myoung34/github-runner](https://github.com/myoung34/docker-github-actions-runner)
- **Ephemeral** by default — the runner gets a clean workspace for every job
- **No Docker socket** — the host socket is not mounted, because it would give jobs root on the runner VM. HIL tests do not need Docker.
- Go and Task are pre-installed in the image; OpenTofu is installed via `setup-opentofu` in the workflow
