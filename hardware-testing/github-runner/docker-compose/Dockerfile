# Tracks the latest runner build for Ubuntu 24.04. Build with --pull to refresh
# this moving tag; upstream's :latest tag uses Ubuntu 20.04 instead.
FROM myoung34/github-runner:ubuntu-noble

ARG GO_VERSION=1.25.0
ARG TASK_VERSION=3.40.1

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

# OpenTofu, from its signed apt repository, via the official installer.
# Installed here rather than with setup-opentofu in each job, which downloads
# a release every time and has failed on transient network errors. Each HIL
# run has several jobs, each in a fresh container.
RUN curl --proto '=https' --tlsv1.2 -fsSL https://get.opentofu.org/install-opentofu.sh -o /tmp/install-opentofu.sh \
    && sh /tmp/install-opentofu.sh --install-method deb \
    && rm -f /tmp/install-opentofu.sh \
    && rm -rf /var/lib/apt/lists/* \
    && tofu version

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
