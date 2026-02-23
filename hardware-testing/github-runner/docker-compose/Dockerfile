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
