# SECSIM Design Scaffold

This folder now contains a runnable TSX + Go scaffold derived from the reference mock and spec.

## Layout

- [spec.md](/home/dev/projects/SECSIM/design/spec.md): original product/UI spec
- [UI_design.tsx](/home/dev/projects/SECSIM/design/UI_design.tsx): original monolithic mock
- [implementation_plan.md](/home/dev/projects/SECSIM/design/implementation_plan.md): phased delivery tracker and current implementation status
- `backend/`: Go API and in-memory simulator state
- `frontend/`: React + TypeScript implementation

## Run

Backend:

```bash
cd design/backend
go run ./cmd/secsimd
```

Frontend:

```bash
cd design/frontend
npm install
npm run dev
```

Vite proxies `/api` to `http://localhost:8080` during development.

## Build

```bash
cd design/backend
go test ./...
cd ../frontend
npm install
npm test
npm run build
```

If `frontend/dist` exists, the Go server will serve the built app at `/`.

Frontend unit tests run with Vitest and Testing Library via `npm test`.

## HSMS Smoke Test

With the backend running on `http://127.0.0.1:8080` in passive HSMS mode, you can run a small client that:

- starts the simulator via `/api/sim/start`
- opens an HSMS session
- sends `S1F13`
- sends `S2F41 TRANSFER`
- prints the `S1F14`, `S2F42`, and `S6F11` responses

```bash
cd design/backend
go run ./cmd/hsmsprobe
```

Optional flags:

```bash
go run ./cmd/hsmsprobe --api http://127.0.0.1:8080 --carrier CARR001 --source LP01
```

## Air-Gapped Package

From the repository root:

```powershell
.\build_airgapped.ps1
```

The full packaging flow is documented in [BUILD_AIRGAPPED.md](/home/dev/projects/SECSIM/design/BUILD_AIRGAPPED.md).

The packaged Windows folder includes `stocker-sim.yaml` next to `secsim.exe`, and the backend now creates that config automatically on first launch if it is missing.
