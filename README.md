# SimpleHermes

> [!WARNING]
> SimpleHermes is in active development and is currently a non-functional pre-alpha.
> Do not rely on it for real station operation yet, and expect breaking changes while the radio, audio, and validation work is still in progress.

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
- Hermes protocol 1 session transport for Hermes Lite 2 class devices
- Basic backend DSP with RX audio playback and TX microphone capture paths
- Local-mode and server-mode radio state management
- Client-mode proxying to a remote SimpleHermes server
- Audio WebSocket transport for local and remote control paths
- Configurable microphone and speaker device preferences where the desktop webview exposes device selection
- Optional shared access key for the external server-mode remote API
- Keyboard shortcuts for common operating actions

Not finished yet:

- Protocol 2 / alternate transport support
- Real on-air validation with a complete station workflow

## Keyboard Shortcuts

- `P`: cycle power presets
- `B`: cycle band presets
- `Shift + B`: read the current band
- `Shift + F`: read the current frequency
- `M`: cycle operating modes
- `Wheel`: tune up or down by the current step
- `Arrow Up` / `Arrow Down`: tune by one current step
- `]`: tune up one step
- `[`: tune down one step
- `Shift + ]`: tune up ten steps
- `Shift + [`: tune down ten steps
- `R`: toggle receive
- `T`: toggle transmit arm
- `S`: open settings
- `H`: read the shortcut list
- `D`: open the debug console
- `Hold Space`: key PTT while held

Accessibility mode can announce state changes through the desktop screen reader stack.

## Debugging

Press `D` or use the `Debug` button in the title bar to open the debug console. It shows live Hermes transport counters, the last TX/RX frequency frames sent to the radio, RX packet and audio-frame counts, WebSocket state, Web Audio state, microphone state, and recent frontend command/audio events.

If the app discovers and connects to a Hermes Lite 2 but there is no audio, open the debug console and check:

- `RX packets` and `RX audio frames` should increase after connection.
- `RX socket` should be `open`.
- `Audio context` should be `running`; use `Start audio` if it is still suspended.
- `Last RX freq` and `Last TX freq` should change when switching bands or entering a frequency.

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

When exposing `server` mode beyond localhost, set a remote access key in Settings and restart the app so the external API listener requires bearer authentication. Client mode uses the same saved key when proxying state, commands, and audio WebSockets to that server.

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

## Credits

SimpleHermes is being developed with protocol, behavior, and workflow reference material from these open-source projects:

- [Thetis](https://github.com/TAPR/OpenHPSDR-Thetis)
- [piHPSDR](https://github.com/g0orx/pihpsdr)
- [LinHPSDR](https://github.com/pa3gsb/linhpsdr)

These projects are being used as development references for interoperability and feature scoping. SimpleHermes is a separate codebase with a much narrower product target.
