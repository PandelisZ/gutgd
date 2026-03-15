# gutgd

`gutgd` is a Wails v3-alpha desktop harness for manually exercising the `gut` Go rewrite from a single window.

It exposes the major `gut` feature areas:

- environment and capability diagnostics
- keyboard input
- mouse movement, click, scroll, and drag
- screen size, capture, highlight, and color inspection
- window listing, focus, move, and resize
- color and window search / wait / assert flows
- clipboard utilities

It also reports feature areas that are currently unavailable in the default `gut` registry:

- image search
- text search
- window element inspection
- window minimize / restore are exposed in the UI but are currently marked unsupported by the native capability report

## Project layout

- `main.go` wires the Wails application and registers the backend service.
- `backend/` contains the Go service that wraps `gut`.
- `frontend/` contains the React + Vite + Fluent UI v9 application.

## Run in development

Windows PowerShell:

```powershell
cd .\gutgd
wails3 dev -config .\build\config.yml
```

## Frontend details

- React Router uses hash-based routes so the desktop shell and the Vite dev server resolve the same paths cleanly.
- The frontend waits for the Wails bridge before calling backend methods, so startup does not fail with `window.wails.Call is unavailable`.
- Fluent UI React v9 is provided by `@fluentui/react-components`.
- Frontend package management uses `pnpm`.

## Generate frontend bindings

```powershell
cd .\gutgd
wails3 generate bindings
```

## Build

Windows PowerShell:

```powershell
cd .\gutgd
task build
```

If `task` is not installed in your shell, run the equivalent commands directly:

```powershell
cd .\gutgd\frontend
pnpm install
pnpm build

cd ..\
go test .\...
go build -o .\build\bin\gutgd.exe .
```

If the `task` binary is not installed in your shell, run the equivalent commands directly:

```powershell
cd .\gutgd\frontend
pnpm install
pnpm build

cd ..\
go build -o .\build\bin\gutgd.exe .
```

## Notes

- Captures are saved under `.\.artifacts\` relative to the app working directory.
- The UI drives live desktop actions. Mouse, keyboard, clipboard, screen, and window operations affect the real host session.
- This project pins Wails to `v3.0.0-alpha.74` in `go.mod`.
- The repo-local `gut` module is linked through `replace gut => ../gut`.
