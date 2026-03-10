# SECSIM Design Implementation Plan

## Goal

Turn the current `design/` reference assets into a runnable scaffold that uses:

- `frontend/`: React + TypeScript + Vite (`.tsx`)
- `backend/`: Go HTTP API with an in-memory simulator state

The existing [UI_design.tsx](/home/dev/projects/SECSIM/design/UI_design.tsx) and [spec.md](/home/dev/projects/SECSIM/design/spec.md) remain the design references. The new scaffold is the implementation baseline.

## Deliverables

- A Go service that exposes stable JSON endpoints for the simulator shell
- A TSX app that renders the toolbar, rules editor, state view, HSMS config, message monitor, and status bar
- In-memory editing flows for rules and HSMS/device config
- Save/reload semantics for config state inside the backend store
- Local docs for running the scaffold

## Architecture

### Backend

- `cmd/secsimd`: entrypoint for the API server
- `internal/model`: shared Go structs for rules, messages, state, and runtime data
- `internal/store`: concurrency-safe in-memory state, seeded from mock data
- `internal/api`: HTTP handlers for bootstrap, runtime control, config, and rule mutations

### Frontend

- `src/App.tsx`: shell composition and data orchestration
- `src/components/*`: focused UI sections instead of one monolithic mock component
- `src/lib/api.ts`: fetch wrapper for the Go API
- `src/lib/ruleToYaml.ts`: lightweight YAML export for a single rule
- `src/styles.css`: shared tokens and panel styling

## API Scope

- `GET /api/health`
- `GET /api/bootstrap`
- `POST /api/runtime/toggle`
- `POST /api/config/save`
- `POST /api/config/reload`
- `POST /api/log/clear`
- `PUT /api/hsms`
- `PUT /api/device`
- `POST /api/rules`
- `PUT /api/rules/:id`
- `DELETE /api/rules/:id`
- `POST /api/rules/:id/duplicate`
- `POST /api/rules/:id/move`

## Non-Goals For This Scaffold

- Real HSMS socket transport
- YAML file persistence on disk
- Drag-and-drop rule ordering
- Full message parsing or protocol execution

Those stay deferred behind the scaffolded API boundaries.

## Verification

1. `go test ./...` from `design/backend`
2. `npm install`
3. `npm run build` from `design/frontend`

