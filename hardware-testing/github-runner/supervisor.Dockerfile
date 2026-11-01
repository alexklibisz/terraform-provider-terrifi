# The `supervisor` compose service: the Docker CLI, plus curl and jq for
# registering runners through the GitHub API (see supervise.sh).
FROM docker:29-cli

RUN apk add --no-cache curl jq
