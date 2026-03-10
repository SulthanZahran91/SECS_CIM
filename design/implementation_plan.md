# SECSIM Design Implementation Plan

Last updated: 2026-03-10

## Purpose

This file is the delivery tracker for the `design/` implementation.

Current reality:

- The repository contains a working TSX + Go scaffold.
- The UI shell, API surface, tests, and packaging flow exist.
- File-backed config persistence, backend rule/runtime execution, and live frontend runtime subscriptions now exist.
- Real HSMS transport and SECS-II codec support are not implemented yet.

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
| 3. HSMS Transport | Pending | No real listener/client/session implementation exists |
| 4. SECS-II Codec + Message Pipeline | Pending | No real frame parsing, encoding, or live pipeline exists |
| 5. Live UI Integration | Partial | UI now follows live backend runtime snapshots, but it is still waiting on real HSMS traffic and remaining monitor polish |
| 6. Packaging + Acceptance | Partial | Air-gapped packaging exists, but only for the scaffolded runtime |

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
- Matched rules generate immediate reply records plus delayed event/mutate actions
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

Status: `Pending`

Planned:

- Add a backend `internal/hsms` package
- Support passive listen mode
- Support active connect mode
- Track socket/session lifecycle
- Implement HSMS control messages:
  - `Select.req` / `Select.rsp`
  - `Deselect.req` / `Deselect.rsp`
  - `Linktest.req` / `Linktest.rsp`
  - `Separate.req`
- Honor configured port, IP, session ID, device ID, and timers

Exit criteria:

- The runtime toggle starts and stops a real HSMS transport
- The status bar reflects actual connection/session state

### Phase 4. SECS-II Codec and Message Pipeline

Status: `Pending`

Planned:

- Encode/decode HSMS frames
- Encode/decode SECS-II headers and body items
- Parse inbound messages into internal message records
- Render decoded and raw SML from live traffic
- Implement auto-response behavior for:
  - `S1F13` / `S1F14`
  - `S1F1` / `S1F2`
  - `S2F25` / `S2F26`

Exit criteria:

- Live traffic appears in the message log and can drive the rule engine

### Phase 5. Live UI Integration

Status: `Partial`

Done:

- UI layout and editor flows are implemented
- Message detail pane, matched-rule view, and status bar exist
- Frontend bootstrap and runtime views now follow live backend snapshot updates over the event stream

Remaining:

- Drive the live monitor from real HSMS session traffic instead of the current decoded runtime injection path
- Surface runtime errors and connection loss clearly
- Add monitor auto-scroll behavior and other sustained-traffic polish against live sessions

Exit criteria:

- UI can be left open during a real HSMS session and reflects the live simulator state

### Phase 6. Packaging, Samples, and Acceptance

Status: `Partial`

Done:

- Windows air-gapped packaging exists
- Built frontend can be served by the backend

Remaining:

- Package sample YAML configs
- Package sample scenarios or golden test fixtures
- Add end-to-end acceptance tests for a live protocol session
- Validate packaged behavior on a clean Windows host
- Confirm offline rebuild workflow on a dependency-prepared machine

Exit criteria:

- A packaged Windows build can run the simulator, load config, and serve the full UI offline

## Current Gaps

The biggest missing pieces are:

1. Real HSMS socket/session handling
2. Real SECS-II encode/decode
3. Wiring protocol-decoded live traffic into the existing runtime controller instead of the current decoded injection path
4. End-to-end acceptance coverage for live protocol sessions

## Recommended Next Step

Implement the transport integration path next:

1. Add an `internal/hsms` session/connection layer
2. Feed decoded frames into the completed Phase 2 runtime controller
3. Wire standard auto-responses alongside rule-driven replies
4. Add protocol-level acceptance tests around a simulated command flow

That connects the completed Phase 2 runtime core to real protocol traffic.
