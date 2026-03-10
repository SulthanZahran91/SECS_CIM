# Air-Gapped Build Guide

Build a Windows package that contains:

- the Go backend executable
- the built TSX frontend under `web/dist`
- helper scripts for offline startup

## Quick Start

From the repository root:

```powershell
.\build_airgapped.ps1
```

Or from `design/` directly:

```powershell
.\build-airgapped.ps1
```

## Common Options

```powershell
.\build_airgapped.ps1 -Port 8080
.\build_airgapped.ps1 -Compress
.\build_airgapped.ps1 -SkipDeps
.\build_airgapped.ps1 -OutputDir C:\Builds
```

## Output

The script writes to `design/dist/` by default and creates:

- a versioned Windows executable
- a `secsim-airgapped-YYYYMMDD/` deployment folder
- an optional zip archive when `-Compress` is used

## Package Layout

```text
secsim-airgapped-YYYYMMDD/
├── secsim.exe
├── start.bat
├── README.txt
└── web/
    └── dist/
```

`secssim.exe` automatically looks for frontend assets in `web/dist` next to the executable. You can also override this with `SECSIM_WEB_DIST`.

## Offline Notes

Connected machine preparation:

1. Run `npm ci` in `design/frontend`
2. Optionally vendor Go modules in `design/backend` with `go mod vendor`
3. Transfer the repository to the offline build machine

Offline build machine:

1. Run `.\build_airgapped.ps1 -SkipDeps`

## Runtime

`start.bat` sets `SECSIM_ADDR` to the selected port if it is not already defined, then starts `secssim.exe`.

Default URL:

```text
http://localhost:8080
```

