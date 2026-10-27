# Go and OpenTofu come from their official images, pinned by tag so that
# Dependabot (the `docker` entry in .github/dependabot.yml) opens PRs to bump
# them. HIL jobs use these instead of downloading their own in every job.
# If go.mod needs a newer Go than this, Go downloads that toolchain itself
# (GOTOOLCHAIN=auto) until the image is bumped and rebuilt.
FROM golang:1.27.3-bookworm AS go
FROM ghcr.io/opentofu/opentofu:1.13.1-minimal AS tofu

# Tracks the latest runner build for Ubuntu 24.04. Build with --pull to refresh
# this moving tag; upstream's :latest tag uses Ubuntu 20.04 instead.
FROM myoung34/github-runner:ubuntu-noble

# Task has no official image, so bump this by hand.
ARG TASK_VERSION=3.54.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=tofu /usr/local/bin/tofu /usr/local/bin/tofu

# The HIL controller's self-signed certificate, fetched by rsync.sh. Go and curl
# read the system trust store, so jobs verify the controller's TLS.
COPY controller.crt /usr/local/share/ca-certificates/terrifi-hil-controller.crt
RUN update-ca-certificates

# Task (taskfile.dev)
RUN sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin "v${TASK_VERSION}"

# Jobs run as the unprivileged `runner` user (`--user runner` in
# supervise.sh). The base image gives that user passwordless sudo, which would
# let job code become root in the container and widen its reach toward the
# runner VM's kernel. Remove it, and fail the build if it comes back in a
# future base image.
RUN gpasswd -d runner sudo \
    && sed -i '/NOPASSWD/d' /etc/sudoers \
    && rm -f /etc/sudoers.d/runner \
    && ! id -nG runner | grep -qw sudo \
    && ! grep -rqs NOPASSWD /etc/sudoers /etc/sudoers.d
