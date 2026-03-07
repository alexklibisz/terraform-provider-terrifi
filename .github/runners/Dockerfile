FROM myoung34/github-runner:ubuntu-noble

ARG GO_VERSION=1.25.0
ARG TASK_VERSION=3.40.1
ARG OPENTOFU_VERSION=1.9.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        git \
        jq \
        unzip \
    && rm -rf /var/lib/apt/lists/*

# Go
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Task (taskfile.dev)
RUN sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin "v${TASK_VERSION}"

# OpenTofu
RUN curl -fsSL "https://github.com/opentofu/opentofu/releases/download/v${OPENTOFU_VERSION}/tofu_${OPENTOFU_VERSION}_$(dpkg --print-architecture).deb" \
        -o /tmp/tofu.deb \
    && dpkg -i /tmp/tofu.deb \
    && rm /tmp/tofu.deb
