# SimpleHermes

SimpleHermes is a focused desktop control app for the Hermes Lite 2 SDR.

The project is deliberately narrow: one radio target, three operating modes, accessible keyboard-first control, and no waterfall or oversized DSP feature surface. The goal is a dependable control panel that works well for blind operators and still stays practical for anyone who wants a clean station interface.

## Current Scope

- Hermes Lite 2 only
- Single desktop binary built from Go
- `local`, `server`, and `client` modes in one application
- Accessible desktop UI with screen-reader-friendly semantics
- Basic radio control: discovery, connect, band selection, operating mode selection, direct frequency entry, step tuning, power presets, RX/TX arm, and PTT
- Remote control path where a server-mode station exposes an HTTP API and a client-mode station controls it

## Current Status

Implemented today:

- Wails desktop shell for Windows and Linux
- Persistent application settings
- HPSDR protocol discovery for Hermes devices on the local network
- Local-mode and server-mode radio state management
- Client-mode proxying to a remote SimpleHermes server
- Keyboard shortcuts for common operating actions

Not finished yet:

- Low-level Hermes Lite 2 command/streaming engine
- Receive audio path
- Transmit audio path
- Real on-air validation with a complete station workflow

## Keyboard Shortcuts

- `P`: cycle power presets
- `B`: cycle band presets
- `M`: cycle operating modes
- `]`: tune up one step
- `[`: tune down one step
- `Shift + ]`: tune up ten steps
- `Shift + [`: tune down ten steps
- `R`: toggle receive
- `T`: toggle transmit arm
- `Hold Space`: key PTT while held

Accessibility mode can announce state changes through the desktop screen reader stack.

## Build

### Linux

Ubuntu 24.04 builds require GTK 3 and WebKitGTK 4.1 development packages:

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
./scripts/build-linux-local.sh
```

The output binary is written to `dist/simplehermes-linux-amd64`.

### Windows

Build with Go:

```bash
go build -tags production -o dist/simplehermes-windows-amd64.exe ./cmd/simplehermes
```

## Run

```bash
./dist/simplehermes-linux-amd64
```

Optional config override:

```bash
./dist/simplehermes-linux-amd64 -config /path/to/config.json
```

Default config path:

- Linux: `$XDG_CONFIG_HOME/simplehermes/config.json` or `~/.config/simplehermes/config.json`
- Fallback when no user config directory is available: `./simplehermes.json`

## Modes

- `local`: control a Hermes Lite 2 visible on the local network
- `server`: same as local, plus expose the remote API on the configured listen address
- `client`: connect to a remote SimpleHermes server and control that station

The current default listen address is `127.0.0.1:8787`.

## Development

Run tests with:

```bash
go test ./...
```

GitHub Actions is configured to test and build Linux and Windows artifacts.

## Project Layout

- [`cmd/simplehermes`](/home/michael/projects/simplehermes/cmd/simplehermes): desktop entrypoint and embedded frontend
- [`internal/app`](/home/michael/projects/simplehermes/internal/app): app state, commands, shortcuts, local/server/client behavior
- [`internal/radio/hpsdr`](/home/michael/projects/simplehermes/internal/radio/hpsdr): HPSDR discovery and Hermes driver scaffolding
- [`internal/web`](/home/michael/projects/simplehermes/internal/web): API server used for server mode and remote client control
- [`scripts`](/home/michael/projects/simplehermes/scripts): local build helpers

## Design Intent

SimpleHermes is not trying to compete with large SDR suites. The target is a compact, predictable operating surface with strong keyboard support and clear spoken feedback when accessibility mode is enabled.
