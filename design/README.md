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

## Air-Gapped Package

From the repository root:

```powershell
.\build_airgapped.ps1
```

The full packaging flow is documented in [BUILD_AIRGAPPED.md](/home/dev/projects/SECSIM/design/BUILD_AIRGAPPED.md).
