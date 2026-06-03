# testing/

Bootstrap scripts for the `uos` acceptance-test target — a third option
alongside the existing `docker` (default) and `hardware` targets. UOS Server
is Ubiquiti's first-party packaging of the Network application; using it
exercises code paths the docker simulation (`linuxserver/unifi-network-application`)
does not, particularly around firewall zones/policies, WLAN, and client groups.

The scripts are designed to run **as root on a Linux host you control** — a
Proxmox LXC, a VM, or bare metal. `install-uos-server.sh` provisions UOS
Server itself (version-pinned); the others bootstrap it and drive the test
target.

> **This is a local / self-hosted target, not github-hosted CI.** See
> "Why not github-hosted runners?" below — the UOS installer is built for a
> bare-root host and does not run cleanly on github-hosted `ubuntu-24.04`
> runners. The `docker` target remains the community-runnable CI fast path;
> `uos` is for running the full hardware-gated suite against a real Network
> application on a host you own.

## Scripts

- `bootstrap.sh` — Walks a fresh UOS Server through first-run setup, creates
  an integration API key on the admin account, and prints
  `UNIFI_API=…`, `UNIFI_API_KEY=…`, `UNIFI_INSECURE=true` on stdout. Idempotent
  — re-running on an already-set-up controller skips the setup step. Pass
  `ENABLE_FAKE_DEVICES=1` to chain into `enable-simulation.sh`.

- `enable-simulation.sh` — Stops uosserver, edits `system.properties` to set
  `is_simulation=true` + `demo.num_ugw`/`usw`/`uap`, restarts uosserver, and
  polls + adopts the synthesized devices via the Network API. Required for
  any test that depends on an adopted gateway or AP (firewall zones, AP
  groups, port profiles, traffic routes). Idempotent.

- `verify-devices.sh` — Post-condition: asserts the controller has the
  expected number of adopted devices. Useful in CI to fail fast if
  simulation didn't fully synthesize.

- `reset-controller.sh` — Wipes the Network app's data volume (mongo,
  `system.properties`, keystore) and restarts uosserver. Preserves UOS-level
  state (admin account, API keys). Use between acceptance-test runs to undo
  whatever state previous tests left behind. After it returns, run
  `bootstrap.sh` again to obtain a fresh integration API key (the previous
  key references the now-wiped Network site and may not work).

## Typical use

```sh
# 1. Install UOS Server (pinned version, SHA256-verified). Idempotent.
sudo bash testing/install-uos-server.sh

# 2. Bootstrap admin + API key, and adopt simulated devices. Prints the
#    UNIFI_* env vars on stdout; eval them into the current shell.
eval "$(ENABLE_FAKE_DEVICES=1 sudo testing/bootstrap.sh)"

# 3. Run the acceptance suite against UOS (same host, or any host that can
#    reach UOS over the network — just carry the UNIFI_* vars across).
export UNIFI_SITE=default
task test:acc:uos
```

The three env vars (`UNIFI_API`, `UNIFI_API_KEY`, `UNIFI_INSECURE`) are the
same ones the existing `hardware` target consumes, so the same provider code
runs against either.

## Fast local dev loop: Proxmox LXC snapshots

If your UOS Server is hosted in a Proxmox LXC and the storage backend is ZFS
or any other snapshot-capable driver, snapshot/rollback is dramatically
faster than running `reset-controller.sh + bootstrap.sh + enable-simulation.sh`
between test runs. The scripts here remain the canonical CI path (no
Proxmox dependency); snapshots are a local-dev convenience.

```sh
# One-time, after the initial bootstrap.sh + ENABLE_FAKE_DEVICES=1:
pct snapshot <VMID> clean --description "uosserver bootstrapped + 6 simulated devices adopted"

# Between test runs:
pct rollback <VMID> clean   # ~2 s; controller comes back already-bootstrapped

# To update the baseline (e.g. after upgrading uosserver):
pct snapshot <VMID> clean-$(date +%Y%m%d) --description "..."  # keep history
pct delsnapshot <VMID> clean     # only after the new one is proven
pct snapshot <VMID> clean
```

Snapshots are a local-dev convenience only; they are not part of any CI
flow (there is no github-hosted UOS CI — see below).

## Why not github-hosted runners?

We investigated running this target on github-hosted `ubuntu-24.04`
runners (a `ci-uos-probe.yaml` workflow, 8 dispatched runs) and concluded
it is not viable without owning the runner. The UOS installer is built for
a bare-root host and derives the "invoking user" from the audit loginuid /
cwd owner — which on a github-hosted runner is the non-root `runner`
account, and is immutable under `sudo` and unaffected by
`systemd-run --scope`. Its rootless-podman steps then target
`/home/runner/.config/<subsystem>`, a 0700 home the `uosserver` service
user can neither traverse nor write. Five layered workarounds each cleared
one subsystem and surfaced the next:

1. scripts must be executable (`git update-index --chmod=+x`) or invoked
   as `sudo bash <script>`;
2. `sudo` keeps `HOME=/home/runner` → `sudo -H`;
3. installer reads `SUDO_USER`/`LOGNAME` → pin both to root;
4. `storage.conf` stat denied under runner's 0700 home → pre-seed a
   traversable config dir;
5. podman picks the runner's stale system `/usr/bin/pasta` over UOS's
   bundled v2026 → set `helper_binaries_dir`.

…and the loginuid-driven `/home/runner/<subsystem>` writes (cni, …) never
stopped. The viable CI paths, if github-hosted-fresh-install is ruled out:

- a **self-hosted runner running as real root** (mirrors the existing
  `ci-hil.yaml` setup) — no `runner`-user conflict;
- a **pre-baked UOS VM image / snapshot** the workflow boots instead of
  installing fresh.

Until one of those exists, `uos` is a **local / self-hosted target**: run
the steps in "Typical use" on a host you control. The `docker` target
(`task test:acc`) remains the github-hosted community CI path.

## Known limitation: firewall zones require real adopted hardware

The `terrifi_firewall_zone`, `terrifi_firewall_policy`, and
`terrifi_firewall_policy_order` resources are gated `requireHardware(t)`
in their test files for a reason that is **not specific to the
provider** — it is a controller-side gap in simulation mode.

The v2 `/firewall/zone` endpoint requires the "Hotspot" default zone
to exist before any user-created zone can be added. Real adopted UGWs
trigger the controller to seed the six default zones (Internal,
External, DMZ, Hotspot, IoT, VPN) automatically. The Network app's
built-in simulation mode does NOT seed them, even after a fully
adopted simulated UGW reaches `state=1` (Connected). Tried and ruled
out:

- Creating a guest-purpose network (no effect).
- Creating a guest WLAN (`is_guest=true`).
- Direct POST of a zone with `default_zone:true` / `zone_key:"Hotspot"`
  / `defaultZone:true` / `zoneKey:"Hotspot"` (all four field shapes
  rejected by the create-zone schema).
- Hypothetical `/firewall/zone/init` endpoint (returns 405).
- Toggling a settings flag (no firewall/zone setting key exists).

Confirmed on UOS Server 5.0.8 (Network app `linuxserver/unifi-network-application:10.3.58`)
and UOS Server 5.1.15 (Network app `10.4.57`).

These tests therefore remain hardware-only and run only via the
`TERRIFI_ACC_TARGET=hardware` target against a real UniFi deployment.
The `uos` target validates everything else.

## Defaults

- Admin credentials: `admin` / `TerrifiTest!2026` — override via `ADMIN_USER`
  and `ADMIN_PASS` if your environment requires it.
- API key name: `terrifi-test` — repeated bootstrap runs append a timestamp
  suffix rather than clobbering prior keys.
- Simulation device counts: 1 UGW, 2 USW, 3 UAP — override via `NUM_UGW`,
  `NUM_USW`, `NUM_UAP`.
