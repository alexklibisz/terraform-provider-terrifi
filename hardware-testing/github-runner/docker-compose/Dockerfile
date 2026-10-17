# Tracks the latest runner build for Ubuntu 24.04. Build with --pull to refresh
# this moving tag; upstream's :latest tag uses Ubuntu 20.04 instead.
FROM myoung34/github-runner:ubuntu-noble

ARG GO_VERSION=1.25.0
ARG TASK_VERSION=3.40.1
ARG TOFU_VERSION=1.13.1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        git \
        jq \
    && rm -rf /var/lib/apt/lists/*

# Go
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Task (taskfile.dev)
RUN sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin "v${TASK_VERSION}"

# OpenTofu. Installed here rather than with setup-opentofu in each job, which
# looks up and downloads a release every time and has failed on transient
# network errors. Each HIL run has several jobs, each in a fresh container.
RUN set -eu; \
    arch="$(dpkg --print-architecture)"; \
    base="https://github.com/opentofu/opentofu/releases/download/v${TOFU_VERSION}"; \
    archive="tofu_${TOFU_VERSION}_linux_${arch}.tar.gz"; \
    cd /tmp; \
    curl -fsSLO "${base}/${archive}"; \
    curl -fsSLO "${base}/tofu_${TOFU_VERSION}_SHA256SUMS"; \
    grep " ${archive}\$" "tofu_${TOFU_VERSION}_SHA256SUMS" | sha256sum -c -; \
    tar -C /usr/local/bin -xzf "${archive}" tofu; \
    rm -f "${archive}" "tofu_${TOFU_VERSION}_SHA256SUMS"; \
    tofu version

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
