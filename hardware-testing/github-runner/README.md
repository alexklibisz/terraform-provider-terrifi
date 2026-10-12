# GitHub Actions Self-Hosted Runner (Docker)

This directory contains a Docker Compose setup for running a GitHub Actions self-hosted runner. The runner is used by the `CI - HIL` workflow to run acceptance tests against real UniFi hardware.

The runner executes code from pull requests. Read [../SECURITY.md](../SECURITY.md) before changing anything here.

## Prerequisites

A server with Docker, Docker Compose, and rsync installed. The runner needs access to the UniFi controller on the local network.

## Creating a GitHub Personal Access Token

The runner uses a fine-grained personal access token to request a short-lived registration token each time it starts.

1. Go to https://github.com/settings/personal-access-tokens/new
2. Name it something descriptive, e.g., `terrifi-gh-runner`
3. Set the resource owner to the repository owner
4. Set an expiration of about a year, and set a reminder to rotate it
5. Under **Repository access**, choose **Only select repositories** and select this repository
6. Under **Permissions**, add **Administration** with **Read and write** access. Metadata is added automatically as read-only. Add nothing else.
7. Click **Generate token** and copy the value

Registering a repository-level runner requires Administration write. That permission also allows changing settings and deleting the repository, so keep the token only on the runner VM. See [../SECURITY.md](../SECURITY.md) for rotation steps.

## Setup

1. Set up SSH access with an alias `terrifi-gh-runner`, i.e., you should be able to run `ssh terrifi-gh-runner` from your host.
2. Rsync the Docker Compose files and `run.sh` to the server: `./rsync.sh`
3. SSH into the server.
4. Copy the env file: `cp .env.example .env` and fill in `REPO_URL` and `ACCESS_TOKEN`.
5. Start the runner: `cd /home/terrifi/docker-compose && ./run.sh`
6. Confirm the runner appears at: https://github.com/<owner>/<repo>/settings/actions/runners

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

Older versions of this setup mounted a `runner-work` named volume that holds
files written by root-run jobs. After deploying the non-root runner, remove it
so stale root-owned files cannot break checkouts:

```sh
docker volume rm docker-compose_runner-work
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
- **Ephemeral** by default — the runner takes one job, deregisters, and registers again when Docker restarts the container. The restarted container keeps its filesystem, so files persist between jobs.
- **Non-root jobs** — jobs run as the unprivileged `runner` user (`RUN_AS_ROOT=false`), and the Dockerfile removes that user's passwordless sudo from the base image. As root, job code could read `ACCESS_TOKEN` from the container's startup environment in `/proc/1/environ`.
- **No Docker socket** — the host socket is not mounted, because it would give jobs root on the runner VM. HIL tests do not need Docker.
- Go and Task are pre-installed in the image; OpenTofu is installed via `setup-opentofu` in the workflow
