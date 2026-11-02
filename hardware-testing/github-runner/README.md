# GitHub Actions Self-Hosted Runner (Docker)

This directory contains a Docker Compose setup for running a GitHub Actions self-hosted runner. The runner is used by the `CI - HIL` workflow to run acceptance tests against real UniFi hardware.

The runner executes code from pull requests. Read [../SECURITY.md](../SECURITY.md) before changing anything here.

## Prerequisites

A server with Docker, Docker Compose, and rsync installed. The runner needs access to the UniFi controller on the local network. The server does not need Tailscale.

A workstation with rsync, OpenSSL, and Tailscale, on the same tailnet as the controller VM (`terrifi-unifi-os-server`). `rsync.sh` fetches the controller's certificate over Tailscale before syncing.

## Creating a GitHub Personal Access Token

The supervisor uses a fine-grained personal access token to register a single-use runner before each job.

1. Go to https://github.com/settings/personal-access-tokens/new.
2. Name it something descriptive, e.g., `terrifi-gh-runner`.
3. Set the resource owner to the repository owner.
4. Set an expiration of about a year, and set a reminder to rotate it.
5. Under **Repository access**, choose **Only select repositories** and select this repository.
6. Under **Permissions**, add **Administration** with **Read and write** access. Metadata is added automatically as read-only. Add nothing else.
7. Click **Generate token** and copy the value.

Registering a repository-level runner requires Administration write. That permission also allows changing settings and deleting the repository, so keep the token only on the runner VM. Runner containers never receive it. See [../SECURITY.md](../SECURITY.md) for rotation steps.

## Setup

1. Set up SSH access with an alias `terrifi-gh-runner`, i.e., you should be able to run `ssh terrifi-gh-runner` from your host. The runner VM is not on the tailnet, so reach it through the Proxmox host with `ProxyJump`.
2. Fetch the controller's certificate and rsync it, the Docker Compose files, and `run.sh` to `~/github-runner` on the server: `./rsync.sh`.
3. SSH into the server.
4. Copy the env file: `cd ~/github-runner && cp .env.example .env` and fill in `REPO_URL` and `ACCESS_TOKEN`.
5. Start the runner: `cd ~/github-runner && ./run.sh`.
6. Confirm the runner appears at: `https://github.com/<owner>/<repo>/settings/actions/runners`.

## Updating the Runner

Run `./rsync.sh` from this directory to sync the files, then run `./run.sh` on
the server to rebuild with the latest `ubuntu-noble` base image and restart the
supervisor. The next job's container uses the rebuilt image. The build uses
`--pull` because Docker otherwise may reuse a cached base image even with
`--build`.

`rsync.sh` also fetches the controller's certificate, and `run.sh` builds it
into the image. Rerun both whenever the controller's certificate changes, for
example after a UniFi OS Server upgrade; until then, HIL jobs fail TLS
verification. To fetch from a different tailnet host, set
`CONTROLLER_TAILNET_HOST` when running `rsync.sh`.

To update directly on the server:

```sh
cd ~/github-runner
./run.sh
```

To follow the runner's output, use `docker compose logs -f supervisor`.

## Reclaiming Disk Space

Each `./run.sh` leaves the previous runner image and several gigabytes of build
cache behind. To clean up, run this on the server:

```sh
cd ~/github-runner
./prune.sh
```

It removes superseded runner images, unused Docker images and build cache older
than 7 days, unused APT packages and cache, archived journals, rotated logs, and
expired temporary files, and reports disk usage before and after. It keeps the
current runner image, Compose containers and networks, and all Docker volumes,
including the Go module cache. Run `./prune.sh --help` for details.

### Moving from `~/docker-compose`

Older versions of this setup lived in `/home/terrifi/docker-compose`, with the
Compose project named `docker-compose`. The files now live in
`/home/terrifi/github-runner`, and the project is named `terrifi-github-runner`. To move an
existing server, run `./rsync.sh`, then on the server:

```sh
cd ~/docker-compose && docker compose down   # stop the old project
mv ~/docker-compose/.env ~/github-runner/.env
cd ~/github-runner && ./run.sh
docker volume rm docker-compose_goproxy-cache  # old Go module cache
rm -rf ~/docker-compose
```

Older versions of this setup mounted a `runner-work` named volume that holds
files written by root-run jobs. After deploying the non-root runner, remove it
so stale root-owned files cannot break checkouts:

```sh
docker volume rm docker-compose_runner-work
```

## Verifying the Runner in GitHub

1. Go to your repository on GitHub.
2. Navigate to **Settings** > **Actions** > **Runners**.
3. The runner should appear with a green "Idle" status.
4. The labels `self-hosted` and `terrifi-hardware-test` should be listed.

## Troubleshooting

### Controller Connection Errors

HIL jobs reach the controller at `https://unifi.local:11443`. Both of these
errors mean the runner needs `./rsync.sh` and then `./run.sh`, either because
it predates the controller TLS setup or because the controller's certificate
changed:

1. `lookup unifi.local on 127.0.0.11:53: server misbehaving`: the job container
   was not started with the `unifi.local` host mapping from `supervise.sh`.
2. `x509: certificate signed by unknown authority`: the runner image does not
   trust the controller's current certificate.

### Note: Idle Runner Not Picking Up Jobs

On October 10, 2026, the HIL job for PR #185 stayed queued with "Waiting for a
runner to pick up this job," while GitHub showed `terrifi-hardware-runner` as
online and idle. We checked that the job's requested labels (`self-hosted` and
`terrifi-hardware-test`) matched the runner's labels. The workflow's repository
guard also evaluated to true, so neither labels nor the guard explained the wait.

The useful clue was in the container logs:

```sh
cd ~/github-runner
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
- **One container per job** — the runner is ephemeral and exits after a job. The `supervisor` compose service (`supervise.sh`) then starts a new container from the image; `docker run --rm` deletes the old one. Docker's restart policies would reuse the old container's filesystem, so caches or config written by an approved fork PR's job could affect later jobs, including the HIL reset that holds the Terraform state credentials.
- **Verified TLS to the controller** — the controller's certificate is self-signed for `unifi.local`. `rsync.sh` fetches it on the workstation over Tailscale, which authenticates the controller VM, and the image trusts it. The runner VM is not on the tailnet, so job code cannot reach it. Runner containers resolve `unifi.local` to the controller's LAN address (`CONTROLLER_LAN_IP`, default `192.168.1.3`) through `--add-host`, so API traffic stays on the HIL LAN and jobs no longer set `UNIFI_INSECURE`.
- **Docker socket and token only for the supervisor** — the socket is equivalent to root on the runner VM. The supervisor runs nothing but `supervise.sh` and exposes no ports.
- **Just-in-time registration** — before each job, the supervisor uses `ACCESS_TOKEN` to request a [JIT runner configuration](https://docs.github.com/en/rest/actions/self-hosted-runners#create-configuration-for-a-just-in-time-runner-for-a-repository), which registers one ephemeral runner. The runner container gets only that configuration and starts the runner directly, bypassing the base image's entrypoint, so `ACCESS_TOKEN` never enters it. After the job, the supervisor deletes the runner in case it never ran one.
- **Non-root jobs** — the runner and its jobs run as the unprivileged `runner` user, and the Dockerfile removes that user's passwordless sudo from the base image.
- **No Docker socket for jobs** — the runner container does not get the host socket, because it would give jobs root on the runner VM. HIL tests do not need Docker.
- Go, Task, and OpenTofu are pre-installed in the image, and HIL jobs use them instead of downloading their own. Go and OpenTofu are copied from their official images, which Dependabot bumps; Task's `TASK_VERSION` is bumped by hand. Run `./run.sh` after merging a bump
- **Go module cache** — the `goproxy` compose service is nginx caching `proxy.golang.org` (`goproxy.conf`). Runner containers join its network and use it through `GOPROXY`, so jobs fetch modules from the local cache instead of the internet; Go falls back to `proxy.golang.org` if the cache errors or times out. Module versions are immutable, so they are cached for 30 days since last use, capped at 2 GB with least-recently-used eviction. Jobs can make the cache fetch modules, but Go checks every module against `go.sum` and the checksum database, so fork code cannot plant modified modules for later jobs. Each job's own module cache still starts empty.
