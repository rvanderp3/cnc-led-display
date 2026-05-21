# CNC LED Display

A Go application that monitors a GRBL-compatible CNC machine over TCP and renders real-time position data on a 32×32 WLED addressable LED matrix.

## Features

- X, Y, Z work/machine position displayed in real time
- Current time (HH:MM) shown on the fourth row
- 2-pixel-wide border on all four sides: green when idle, red when busy/alarmed
- Polls the CNC controller every 500 ms; sends only changed pixels to WLED
- Configurable CNC and WLED addresses via command-line flags
- Exponential backoff on CNC reconnection (1 s → 30 s)
- ASCII art visualization of the LED matrix printed to the console each frame

## Requirements

- Go 1.22.0 or higher
- GRBL-compatible CNC controller with TCP status reporting
- WLED controller driving a 32×32 LED matrix

## Usage

### Run with default settings
```
cnc-led-display.exe
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `192.168.0.41:23` | CNC controller address (host:port) |
| `-wled-addr` | `http://192.168.0.52` | WLED controller base URL |
| `-clear` | — | Clear the display and exit |

### Examples

```
cnc-led-display.exe -addr 192.168.0.42:23
cnc-led-display.exe -wled-addr http://192.168.0.100
cnc-led-display.exe -clear
```

## Display Layout

```
████████████████████████████████
██ X-315.3              ████████
████████████████████████████████
██ Y-134.0              ████████
████████████████████████████████
██ Z35.4                ████████
████████████████████████████████
██ 17:42                ████████
████████████████████████████████
```
*(border shown in status color; inner content not to scale)*

- Rows 1–3: X (red), Y (green), Z (blue) coordinates
- Row 4: current time in HH:MM (white)
- Border (rows/columns 0–1 and 30–31 on all sides): green = Idle, red = anything else

## Build

### Windows
```
build.bat
```

### Manual
```
go build -o cnc-led-display.exe cnc_coords.go
```

## Console output example

Each poll prints a parsed status block followed by an ASCII art render of the 32×32 LED matrix:

```
--- CNC Status ---
State:      Idle
Position:   -315.320,-134.000,35.400
Buffer:     31,1199
Feed/Speed: 0,0
Overlay:    100,100,100
------------------
+--------------------------------+
|********************************|
|** *                          **|
...
+--------------------------------+
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
