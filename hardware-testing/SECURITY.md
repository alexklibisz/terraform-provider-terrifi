# HIL Security Model

The hardware-in-the-loop (HIL) tests run code from pull requests on a self-hosted GitHub Actions runner.
That runner lives on a home lab network and holds credentials for a real UniFi controller.
This document describes what needs protecting, which code can run on the runner, the controls that limit it, and the gaps that remain.

Read this before approving a fork PR's HIL run, changing the HIL workflows, or changing the runner.

## What Needs Protecting

| Asset | Where it lives | Impact if exposed |
|---|---|---|
| UniFi API key | Repository secret `TERRIFI_HIL_TEST_UNIFI_API_KEY` | Admin access to the HIL test controller |
| Heartbeat URL | Repository secret `TERRIFI_HIL_HEARTBEAT_URL` | Someone could fake or suppress the nightly heartbeat |
| Runner registration token | `.env` on the runner VM, as `ACCESS_TOKEN` | Admin access to this repository, including settings and deletion |
| Runner VM and HIL network | A Proxmox VM on the HIL network | A foothold for reaching the controller, the HIL network, and the VM's tailnet |

## Which Code Runs on the Runner

| Trigger | Code that runs | Approval |
|---|---|---|
| Nightly cron | `main` | None |
| Manual dispatch | The selected branch | None, requires write access |
| PR from a branch in this repository | The PR head commit | None, requires write access |
| PR from a fork | The PR head commit | Required, per push |
| Device-types site deploy (`deploy-site.yaml`), on a push to `main` that touches the site, or on manual dispatch | `main`, or the selected branch | None, requires write access |

Fork PRs also trigger the `pull_request` event. The workflow skips that path, because GitHub withholds secrets from it.

Dependabot PRs run on the same-repository path. GitHub gives Dependabot-triggered runs only Dependabot secrets. The HIL job therefore fails at the connection check, unless the UniFi key is also stored as a Dependabot secret.

## Fork PR Flow

Fork PRs go through `pull_request_target` in [`ci-hil.yaml`](../.github/workflows/ci-hil.yaml). The test steps live in the reusable [`ci-hil-run.yaml`](../.github/workflows/ci-hil-run.yaml).

1. Opening, reopening, or pushing to a fork PR starts a run. GitHub always loads `pull_request_target` workflows from `main`, so the PR cannot change the workflow, the approval gate, or which secrets are passed.
2. The `Fork PR approval` job runs on a GitHub-hosted runner and waits on the `hil-fork-approval` environment. It holds no secrets, and a waiting run does not occupy the hardware runner.
3. A maintainer reviews the PR and approves the deployment. The job logs the PR number, fork repository, and head commit SHA.
4. The HIL job checks out that exact SHA and runs the tests with the UniFi key. The result appears as a check on the PR.
5. A new push to the PR cancels the older run, whether it is waiting for approval or already running. The new run needs its own approval.

Other controls on this path:

- **Read-only token:** the workflow sets `permissions: contents: read`. By default, `pull_request_target` would grant a write token.
- **No stored git credentials:** checkout uses `persist-credentials: false`.
- **Explicit opt-in to fork checkout:** `actions/checkout` refuses fork code under `pull_request_target` unless `allow-unsafe-pr-checkout` is set. Only the fork path sets it, and only after approval.
- **No shared Actions caches:** `setup-go` caching is disabled, so fork code cannot write caches that trusted runs on `main` restore.
- **Minimal secrets:** fork runs receive the UniFi key, but never the heartbeat URL.

### Two Separate Approvals

A fork PR asks the maintainer for two different approvals:

1. **Workflow approval**, shown as a banner on the PR. The repository requires approval for every external contributor before `pull_request` workflows run, for example the Docker-based tests in `CI - Local`. These run fork code on GitHub-hosted runners, with no secrets and a read-only token. This approval does not apply to `pull_request_target`, so it does not gate the HIL run.
2. **Deployment approval** for the `hil-fork-approval` environment, shown on the HIL run itself. This is the gate for the hardware runner.

Approving the first does not approve the second.

### Before Approving a Fork Run

Approving lets the PR's code run on the runner as the `runner` user, with the UniFi key and network access to the HIL controller. Only approve code you have read.

1. Open the waiting run. Confirm that the head SHA matches the commit you reviewed.
2. Read the whole diff at that commit. Pay extra attention to anything that executes during the build or tests:
   - Go code, including `init()` functions, `TestMain`, and test helpers
   - `go.mod` and `go.sum` changes, which can pull in new code
   - `Taskfile.yml`, shell scripts, and anything under `cmd/terrifi-tests`
   - Changes under `.github/` do not take effect for fork runs, because the workflow comes from `main`. Still review them in case the PR is merged.
3. Approve with **Review deployments**. Do not use **Start all waiting jobs**, which is the administrator bypass rather than a review.

## Runner Hardening

The runner is defined in [`github-runner/`](./github-runner/).

- **No Docker socket:** access to the socket is equivalent to root on the runner VM. The HIL tests do not need Docker. Docker-based tests run on GitHub-hosted runners.
- **Non-root jobs without sudo:** the container's first process keeps the registration token in its startup environment, and only root can read that. Jobs therefore run as the unprivileged `runner` user. The base image gives that user passwordless sudo, so the Dockerfile removes it and fails the build if it comes back.
- **Fine-grained registration token:** the runner uses a fine-grained personal access token scoped to this repository. Its only permission is Administration read and write, plus the required Metadata read. Registering a repository-level runner requires Administration write. GitHub offers a narrower "Self-hosted runners" permission, but only for organization runners.
- **Ephemeral registration:** the runner takes one job, deregisters, and registers again when Docker restarts the container.

## Repository Settings

These controls live in GitHub settings rather than in code. If any of them change, update this list.

- **Environment `hil-fork-approval`** (Settings, then Environments):
  - Required reviewers: the maintainer.
  - Deployment branches: `main` only. Fork runs deploy from `main`, so they still pass, and no other branch can use the environment.
  - Environment secrets: none. The UniFi key is a repository secret, passed only to the job after the gate.
- **Actions policy "HIL workflow events"** (Settings, then Actions, then Policies):
  - Target: `.github/workflows/ci-hil.yaml` only.
  - Restrict events: on, with every event allowed.
  - Restrict actors: off. Fork authors trigger the runs, so restricting actors would block exactly the runs this flow exists for.
  - Purpose: starting November 2, 2026, GitHub blocks `pull_request_target` on public repositories unless a policy allows it. Without this policy, fork PRs would get no HIL check at all.
- **Actions settings** (Settings, then Actions, then General):
  - Approval for running fork pull request workflows: require approval for all external contributors.
  - Workflow permissions: read repository contents and packages. Workflows that need more must request it.
  - Allow GitHub Actions to create and approve pull requests: off.
  - Actions permissions: all actions and reusable workflows are allowed, and SHA pinning is not required. See Known Gaps.
- **Repository secrets:** `TERRIFI_HIL_TEST_UNIFI_API_KEY` and `TERRIFI_HIL_HEARTBEAT_URL`.
- **Rulesets:** no ruleset requires the HIL check. Fork PRs can merge without it, at the maintainer's discretion.

## Known Gaps

These are accepted for now. Each one notes what would close it.

1. **Approved code is fully trusted.** It can read and send out the UniFi key, and it can reach the HIL network. Review is the only control. Rotate the key if a run looks suspicious.
2. **State persists between jobs.** Docker restarts the same container after each job, so its filesystem survives. That includes the Go module cache and build cache under `/home/runner`. A fork job could plant files that a later trusted job uses. Closing this needs a fresh container per job, for example a host-side loop that runs `docker run --rm` for each job. A partial fix is pointing the Go caches at the per-job temp directory.
3. **The registration token is on the runner VM.** Jobs cannot read it, but a container escape or a compromised VM can. Its Administration permission includes deleting this repository. Closing this needs the token kept outside the container, for example in the host-side loop. Another option is registering a persistent runner once, with a one-time registration token, so no long-lived token is stored.
4. **Jobs can read the runner's session credentials.** These are scoped to this runner and stop working when the ephemeral registration ends.
5. **The runner VM runs Tailscale.** Job code can reach whatever the VM's tailnet access controls allow. Restrict the VM with a tag-based Tailscale ACL so it can reach only what it needs.
6. **Admins can bypass the environment.** The environment allows administrator bypass. Leave it on only while the maintainer is the only administrator.
7. **Third-party actions are pinned by tag.** The HIL job runs actions such as `opentofu/setup-opentofu` and `arduino/setup-task` with the UniFi key in its environment. If one of those tags were moved to malicious code, it would run on the runner without review. Pinning actions to full commit SHAs, and enabling the setting that requires it, would close this.

## Maintenance

### Rotating the Registration Token

1. Create a fine-grained token at https://github.com/settings/personal-access-tokens/new:
   - Resource owner: the repository owner.
   - Expiration: about a year. Set a reminder to rotate it before then.
   - Repository access: only `terraform-provider-terrifi`.
   - Repository permissions: Administration, read and write. Nothing else.
2. Wait until no HIL job is running, because restarting the container kills an in-flight job.
3. On the runner VM, replace `ACCESS_TOKEN` in `/home/terrifi/docker-compose/.env`, then run `./run.sh`.
4. Under Settings, then Actions, then Runners, confirm the runner is listed as Idle.
5. Run a short manual HIL run twice, starting the second after the first finishes. The second run proves the runner can register again after a job:

   ```sh
   gh workflow run ci-hil.yaml --ref main -f test_filter='^TestAccFirewallGroup_portGroup$'
   ```

6. Revoke the old token.

### If a Run Looks Malicious

1. Cancel the run. Stop the runner by running `docker compose down` in `/home/terrifi/docker-compose` on the runner VM.
2. Revoke the registration token, and create a new one.
3. Rotate the UniFi API key on the UniFi OS Server, and update `TERRIFI_HIL_TEST_UNIFI_API_KEY`.
4. Rotate the heartbeat URL if a trusted run may have been affected.
5. Rebuild the runner from a clean state. Restore a Proxmox snapshot of the VM if one exists. Otherwise, remove the container and rebuild it with `./run.sh`.
6. Review the repository's audit log, Actions runs, environments, policies, and runner list for changes. Check the UniFi controller's configuration as well.
