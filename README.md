# doubletake-bar

A native AirPlay tray menu for [doubletake](https://github.com/omarroth/doubletake),
an AirPlay mirroring sender for Linux. Runs alongside `doubletake -daemonize` and
exports a StatusNotifierItem and `com.canonical.dbusmenu` using
[fyne.io/systray](https://github.com/fyne-io/systray). Waybar's `tray` module renders
its menu with the same GTK theme, hover effects, separators, submenus and
checkboxes as nm-applet. There is no custom popup, positioning code or CSS.

## Run

Run `doubletake-bar tray` once per graphical session, preferably as a systemd user
service. Enable Waybar's `tray` module and remove any `custom/airplay` module.
Right-click the tray icon for the menu:

- Disabled **AirPlay** header and discovered receivers; the mirroring receiver is checked.
  Select an idle receiver to connect, or an active receiver to disconnect.
- **Stop mirroring** disconnects every stream, including connections in progress.
- Per-receiver submenus provide **Disconnect**, **Mute audio** (when audio is available),
  and **Enter PIN…** / **Enter password…** during pairing. Selecting a receiver awaiting
  credentials also opens the prompt. dbusmenu has no entry widget, so credentials
  use a standard Zenity GTK dialog. Cancellation leaves pairing pending.
- **Rescan** requests the daemon's discovery results; doubletake continuously
  browses mDNS in five-second scan windows, expiring receivers after 30 seconds.

The display icon is gray when idle, amber while connecting/pairing, blue while
mirroring, and dimmed when unavailable. Its tooltip describes the current state.
Receiver, state and audio changes arrive as push snapshots; there is no status
polling. The tray reconnects after a daemon restart (one-second retry backoff).

## Daemon event support (required)

Unmodified doubletake v0.5.0 supports only request/response commands. Apply
[`patches/doubletake-watch.patch`](patches/doubletake-watch.patch) to that version.
The patch adds a `watch` request on the existing private Unix control socket,
returning an initial newline-delimited JSON snapshot and a fresh snapshot on
observable changes. Snapshots contain both `devices` and `streams`. Slow clients
retain the latest snapshot; disconnected clients are removed. Existing commands
and their response format are unchanged. The patch includes a subscription test.
The flake exports it as `patches.${system}.default` for downstream packaging:

```nix
doubletake = (final.callPackage ./packages/doubletake.nix { }).overrideAttrs (old: {
  patches = (old.patches or []) ++ [ inputs.doubletake-bar.patches.${laptopSystem}.default ];
});
```

Without the patch the menu reports that the daemon is unavailable; it does not
silently fall back to polling. The socket defaults to
`$XDG_RUNTIME_DIR/doubletake.sock` (or `/tmp/doubletake.sock`).

## CLI compatibility

- `doubletake-bar status` still prints Waybar custom-module JSON once.
- `doubletake-bar popup` is retained as an alias for `tray`. The old layer-shell
  popup and toggle socket have been removed; use the tray icon to open/close the
  native menu. A second tray instance exits with an explanatory error.

## Build and test

```sh
nix build
nix develop
go test -race ./...
go build ./cmd/doubletake-bar
```

The tray is pure Go on Linux (no gotk3 or layer-shell dependencies). The Nix
package wraps Zenity onto PATH for pairing and error dialogs.
