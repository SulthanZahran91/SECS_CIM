# SECSIM Design Implementation Plan

Last updated: 2026-03-13

## Purpose

This file is the delivery tracker for the `design/` implementation.

Current reality:

- The repository contains a working TSX + Go scaffold.
- The UI shell, API surface, tests, and packaging flow exist.
- File-backed config persistence, backend rule/runtime execution, and live frontend runtime subscriptions now exist.
- Real HSMS transport now exists, and a minimal SECS-II codec/message pipeline is wired into the live runtime.
- The live UI now surfaces transport/runtime failures and supports paused vs. auto-tail monitor behavior during sustained sessions.
- The live UI now also provides an operations overview, filterable message workspace, and improved responsive layout for day-to-day simulator operation.
- The rule and HSMS editors now provide starter presets, inline validation, and clearer save-vs-restart guidance during authoring.
- The runtime now distinguishes pending HSMS connection restarts from generic config dirtiness, and idle selected sessions no longer trip false `T8` read timeouts.
- HSMS transport tracing now logs TCP connect/accept/close plus control-frame flow (`Select`, `Deselect`, `Linktest`, `Separate`) for integration debugging.
- Active-mode sessions can now optionally initiate a minimal host-style startup (`S1F13`, `S1F17`, `S2F31`, `S6F12`) for interoperability with equipment-side stacks.
- Protocol coverage is still intentionally narrow: the current implementation focuses on handshake, remote-command, loopback, and event flows.
- Rule-driven outbound actions can now declare generic `SxFy` messages with hand-authored SECS item bodies (`L`, `A`, signed/unsigned integers, `B`, `BOOLEAN`) across the UI, YAML config, runtime logging, and outbound HSMS encoding.

The design references remain:

- [UI_design.tsx](/home/dev/projects/SECSIM/design/UI_design.tsx)
- [spec_in_go.md](/home/dev/projects/SECSIM/design/spec_in_go.md)

## Status Legend

- `Completed`: implemented and verified in the current repo
- `Partial`: implemented only as scaffold/mocks, or missing production behavior
- `Pending`: not implemented yet
- `Deferred`: intentionally left for later

## Progress Summary

| Phase | Status | Meaning |
|---|---|---|
| 0. Scaffold Baseline | Completed | TSX frontend, Go API, docs, packaging, and test harness exist |
| 1. Config + Persistence | Completed | Backend now boots from YAML, saves atomically to disk, and reloads file-backed state |
| 2. Rule Engine + State Mutations | Completed | Decoded inbound runtime messages can match rules, emit replies/events, mutate live state, and stream live snapshots to the UI |
| 3. HSMS Transport | Completed | Real passive/active HSMS runtime now starts, stops, tracks session state, and handles core control traffic |
| 4. SECS-II Codec + Message Pipeline | Completed | Frames and a minimal supported SECS-II item set are encoded/decoded, logged, auto-responded, and fed into the rule engine |
| 5. Live UI Integration | Completed | UI now reflects live HSMS traffic, surfaces transport/update failures, and supports paused/auto-tail monitor behavior |
| 6. Packaging + Acceptance | Partial | Air-gapped packaging exists and protocol acceptance coverage now exercises the real runtime, but packaged Windows validation is still pending |

## Completed Work

### Phase 0. Scaffold Baseline

Status: `Completed`

Implemented:

- React + TypeScript + Vite frontend under `design/frontend`
- Go HTTP backend under `design/backend`
- UI shell for toolbar, rules, state, HSMS config, message monitor, and status bar
- In-memory API for bootstrap, runtime toggle, config updates, and rule CRUD
- Air-gapped build scripts and packaging docs
- Backend and frontend unit tests

Verified:

1. `go test ./...`
2. `go build ./cmd/secsimd`
3. `npm test`
4. `npm run build`
5. Windows cross-build smoke test for `./cmd/secsimd`

## Phase Plan

### Phase 1. Config Schema and Persistence

Status: `Completed`

Done:

- Typed config/state/rule models exist in the backend
- Save/reload endpoints exist
- The frontend edits HSMS/device/rule data through the API
- Single-rule YAML export exists in the frontend
- The backend loads simulator config from YAML on startup
- Save writes the active config to disk with atomic replace semantics
- Reload re-reads YAML from disk, preserves runtime session state, and keeps the last good config on validation failure
- Dirty tracking now compares the working config to the file-backed baseline
- A sample YAML config now ships at `design/backend/stocker-sim.yaml`

Remaining:

- None for the current Phase 1 exit criteria

Exit criteria:

- Simulator boots from a YAML file
- Save writes the active config to disk
- Reload restores disk state instead of mock baseline

### Phase 2. Rule Engine and State Store

Status: `Completed`

Done:

- Rule data structures exist
- Rule CRUD, duplication, and reordering API exists
- UI supports editing conditions, reply templates, and actions
- The backend now keeps a live runtime state store separate from persisted `initial_state`
- The backend can match decoded inbound commands against rules in order
- Matched rules generate immediate reply records plus delayed send/mutate actions
- Send actions can now declare arbitrary outbound `SxFy` messages with raw SECS item payloads instead of only a fixed event shape
- Scheduled mutations update the live state store without dirtying persisted config
- Basic rule-match diagnostics are recorded on inbound message records for matched and near-miss cases
- A simulator controller now wires the rule engine into `/api/sim/start`, `/api/sim/stop`, `/api/sim/status`, and a decoded injection path for backend-driven testing
- Rule conditions can now read decoded message fields directly, alongside the existing state-path and special-predicate checks
- Store mutations now publish live snapshot updates, and the frontend subscribes so runtime replies, events, and state changes appear without manual refresh

Remaining:

- None for the current Phase 2 exit criteria

Exit criteria:

- A decoded inbound command delivered through the runtime controller can match a rule and produce reply, state changes, timed events, and live UI updates

### Phase 3. HSMS Transport

Status: `Completed`

Done:

- Added a backend `internal/hsms` package with real socket/session management
- Runtime start/stop now starts and stops a real HSMS transport instead of only toggling an in-process scheduler
- Passive listen mode now accepts host connections and transitions through live session states
- Active connect mode now dials the configured peer, sends `Select.req`, and retries on disconnect
- Core HSMS control handling now exists for:
  - `Select.req` / `Select.rsp`
  - `Deselect.req` / `Deselect.rsp`
  - `Linktest.req` / `Linktest.rsp`
  - `Separate.req`
- Session state changes now update the shared runtime snapshot so the status bar reflects actual transport state
- The runtime now applies configured address, port, session ID, and basic T5/T6/T7/T8 timer behavior
- `T8` enforcement now applies to inter-byte stalls within a frame instead of idle time between frames, which keeps selected sessions up against quieter hosts
- The backend now emits trace logs for TCP session lifecycle and HSMS control frames so external host handshake issues can be diagnosed from runtime logs
- Active mode now has an optional host-startup path that sends `S1F13`, advances through `S1F17` and `S2F31`, and acknowledges inbound `S6F11` with `S6F12`

Remaining:

- None for the current Phase 3 exit criteria

Exit criteria:

- The runtime toggle starts and stops a real HSMS transport
- The status bar reflects actual connection/session state

### Phase 4. SECS-II Codec and Message Pipeline

Status: `Completed`

Done:

- HSMS frame read/write support now exists with real 10-byte headers and system bytes
- A minimal SECS-II item codec now exists for the currently used item types:
  - `L`
  - `B`
  - `BOOLEAN`
  - `A`
  - `U1`
  - `U4`
- Inbound live traffic is now decoded into internal runtime messages instead of relying only on the decoded injection path
- The backend now renders both pretty and raw SML strings from live protocol bodies for the monitor
- `S2F41` remote-command messages are now decoded into `RCMD` plus named parameter fields for rule matching
- Auto-response behavior now exists for:
  - `S1F13` / `S1F14`
  - `S1F1` / `S1F2`
  - `S2F25` / `S2F26`
- Active-mode host bootstrap now exists for a minimal equipment bring-up sequence:
  - `S1F13` / `S1F14`
  - `S1F17` / `S1F18`
  - `S2F31` / `S2F32`
  - `S6F11` / `S6F12`
- Rule-driven `S2F42` replies and scheduled outbound messages are now encoded and sent over the selected HSMS session
- The SECS-II item codec now supports the hand-authored payload types used by the outbound message editor, including signed integer items and nested list parsing from SML-like text
- Protocol-level tests now cover frame/item round-trips plus a live passive-session command flow through auto-response, rule match, reply, and scheduled event emission

Remaining:

- None for the current Phase 4 exit criteria

Exit criteria:

- Live traffic appears in the message log and can drive the rule engine

### Phase 5. Live UI Integration

Status: `Completed`

Done:

- UI layout and editor flows are implemented
- Message detail pane, matched-rule view, and status bar exist
- Frontend bootstrap and runtime views now follow live backend snapshot updates over the event stream
- The live monitor now updates from real HSMS session traffic, including protocol auto-responses, rule replies, and scheduled events
- Runtime transport failures from the backend are now surfaced directly in the UI and toolbar state
- Live-update stream disconnects now surface as reconnecting warnings in the UI
- The message monitor now supports paused vs. live-tail behavior, plus a jump-to-latest affordance for sustained traffic
- The message workspace now includes client-side search plus direction/source filters so live traffic can be narrowed without losing the live-tail workflow
- The HSMS tab now shows `restart required` only for unapplied connection-setting changes, and that indicator clears after a successful stop/start cycle
- The Rules tab now uses a generic outbound-message editor with explicit `Stream`, `Function`, `W-Bit`, and hand-authored body text instead of a fixed `S6F11` report builder
- The UI now opens with an operations overview strip, richer toolbar status, denser state summaries, and more responsive layout/styling for operator use on narrower screens
- The authoring flows now include rule starter presets, rule readiness feedback, inline HSMS/device validation, and explicit save-versus-restart guidance for connection edits

Remaining:

- None for the current Phase 5 exit criteria

Exit criteria:

- UI can be left open during a real HSMS session and reflects the live simulator state

### Phase 6. Packaging, Samples, and Acceptance

Status: `Partial`

Done:

- Windows air-gapped packaging exists
- Built frontend can be served by the backend
- The Windows package now includes a default `stocker-sim.yaml` config next to `secsim.exe`
- A small HSMS smoke-test client now exists at `design/backend/cmd/hsmsprobe` for manual protocol validation against a running simulator
- Protocol acceptance tests now cover passive-mode control-message handling plus active-mode disconnect/reconnect behavior against the real runtime

Remaining:

- Package sample scenarios or golden test fixtures
- Validate packaged behavior on a clean Windows host
- Confirm offline rebuild workflow on a dependency-prepared machine

Exit criteria:

- A packaged Windows build can run the simulator, load config, and serve the full UI offline

## Current Gaps

The biggest missing pieces are:

1. Broader SECS-II/message coverage beyond the current handshake, control, remote-command, loopback, and event paths
2. Sample scenario or golden-fixture coverage for broader host workflows
3. Packaged Windows validation and offline rebuild confirmation for the real protocol runtime

## Recommended Next Step

Expand coverage around the real protocol runtime:

1. Add the next set of SECS-II message/body shapes needed for the target host scenarios
2. Add sample scenarios or golden transcripts for those host flows so packaged builds can be smoke-tested consistently
3. Validate the packaged Windows build against a clean-host live-session smoke test
4. Confirm the documented offline rebuild workflow on a dependency-prepared machine

That moves the repo from “real transport exists” to “real transport is scenario-complete and release-validated.”
