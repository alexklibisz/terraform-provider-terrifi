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
2. Rsync the Docker Compose files to the server: `./rsync.sh`
3. SSH into the server.
4. Copy the env file: `cp .env.example .env` and fill in `REPO_URL` and `ACCESS_TOKEN`.
5. Start the runner: `docker compose up -d`
6. Confirm the runner appears at: https://github.com/your-org/terrifi/settings/actions/runners

## Verifying the Runner in GitHub

1. Go to your repository on GitHub
2. Navigate to **Settings** > **Actions** > **Runners**
3. The runner should appear with a green "Idle" status
4. The labels `self-hosted` and `terrifi-hardware-test` should be listed

## Architecture

- **Ubuntu Noble (24.04)** base image via [myoung34/github-runner](https://github.com/myoung34/docker-github-actions-runner)
- **Ephemeral** by default — the runner gets a clean workspace for every job
- **Docker socket** is mounted so jobs can use Docker (e.g., testcontainers)
- Go and Task are pre-installed in the image; Terraform is installed via `setup-terraform` in the workflow
