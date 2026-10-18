# Go and OpenTofu come from their official images, pinned by tag so that
# Dependabot (the `docker` entry in .github/dependabot.yml) opens PRs to bump
# them. HIL jobs use these instead of downloading their own in every job.
# If go.mod needs a newer Go than this, Go downloads that toolchain itself
# (GOTOOLCHAIN=auto) until the image is bumped and rebuilt.
FROM golang:1.27.2-bookworm AS go
FROM ghcr.io/opentofu/opentofu:1.13.1-minimal AS tofu

# Tracks the latest runner build for Ubuntu 24.04. Build with --pull to refresh
# this moving tag; upstream's :latest tag uses Ubuntu 20.04 instead.
FROM myoung34/github-runner:ubuntu-noble

# Task has no official image, so bump this by hand.
ARG TASK_VERSION=3.54.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        git \
        jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=tofu /usr/local/bin/tofu /usr/local/bin/tofu

# Task (taskfile.dev)
RUN sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin "v${TASK_VERSION}"

# Jobs run as the unprivileged `runner` user (RUN_AS_ROOT=false in
# docker-compose.yaml). The base image gives that user passwordless sudo, which
# would let job code become root and read the runner's startup environment,
# including ACCESS_TOKEN, from /proc/1/environ. Remove it, and fail the build
# if it comes back in a future base image.
RUN gpasswd -d runner sudo \
    && sed -i '/NOPASSWD/d' /etc/sudoers \
    && rm -f /etc/sudoers.d/runner \
    && ! id -nG runner | grep -qw sudo \
    && ! grep -rqs NOPASSWD /etc/sudoers /etc/sudoers.d
