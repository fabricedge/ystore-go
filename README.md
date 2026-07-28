# ystore-go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/katri/ystore-go)](https://goreportcard.com/report/github.com/katri/ystore-go)

**Hide arbitrary data inside H.264 video files (and their AAC audio track) in a
way that survives video platform re-encoding. Default resolution 1920×1080,
configurable to any size ≥64×64.**

Data is stored as a visual grid of gray-level cells in each video frame,
protected by Reed-Solomon error correction. An optional MFSK audio channel
provides a secondary low-rate carrier.

---

## Install

### From source

```bash
git clone https://github.com/katri/ystore-go.git
cd ystore-go
make build
```

### From release

Download the pre-built binary for your platform from the
[Releases page](https://github.com/katri/ystore-go/releases).

| File | Platform |
|------|----------|
| `ystore-linux-amd64` | Linux x86_64 |
| `ystore-linux-arm64` | Linux ARM64 |
| `ystore-darwin-amd64` | macOS Intel |
| `ystore-darwin-arm64` | macOS Apple Silicon |
| `ystore-windows-amd64.exe` | Windows x86_64 |

**Prerequisite:** [ffmpeg](https://ffmpeg.org/) must be installed and in `PATH`.

---

## Usage

### Encode a single file into a video

```bash
ystore encode --input secret.pdf --output video.mp4
```

This produces an H.264 MP4 (default 1920×1080). Upload it to a video platform,
share it, store it anywhere. The data survives re-encoding.

### Encode multiple files (bundled)

```bash
ystore encode --input doc1.pdf --input doc2.pdf --input doc3.jpg --output bundle.mp4
```

Multiple files are automatically bundled into a tar archive with a magic prefix.
Decoding auto-detects the bundle.

### Encode all files from a directory

```bash
ystore encode --input-dir ./documents --output bundle.mp4
```

### Decode a video back to the original file

```bash
ystore decode --input video.mp4 --output restored.pdf
```

### Decode a multi-file bundle to a directory

```bash
ystore decode --input bundle.mp4 --output-dir ./extracted
```

If the video contains a bundled archive, files are extracted to the given
directory. Single files are written as `output.bin`.

### Encode over an existing video (background)

```bash
ystore encode --input secret.pdf --background some_video.mp4 --output blended.mp4
```

The cell grid is drawn on top of the background video; non-cell areas (borders,
guard bands) show the original background content. The background video must
match the configured `--width` and `--height`; ffmpeg resizes frames to match.

### Encode with custom resolution

```bash
ystore encode --input secret.pdf --width 640 --height 480 --output small.mp4
```

Smaller resolutions produce fewer cells per frame (less data per frame) but
result in smaller video files. Width and height must match on encode and decode.

### With audio redundancy

```bash
ystore encode --input secret.pdf --output video.mp4 --audio
ystore decode --input video.mp4 --output restored.pdf --audio
```

The audio channel carries the same data at a lower rate and can be used as a
backup if the video track is damaged.

### With password encryption

```bash
ystore encode --input secret.pdf --password mypass --output private.mp4
ystore decode --input private.mp4 --password mypass --output restored.pdf
```

Data is encrypted with AES-256-GCM before embedding. The encryption key is
derived from the password via Argon2id (memory-hard KDF) with a random salt.
Each encode uses a unique salt and nonce. Wrong password produces a clear
error: <code>cipher: message authentication failed (wrong password?)</code>

---

## CLI Reference

### ystore encode

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | — | Input file to encode (repeatable: `--input a.pdf --input b.pdf`) |
| `--input-dir` | — | Encode all files in a directory |
| `--output` | `output.mp4` | Output video file |
| `--cell-size` | `24` | Visual grid cell size in pixels |
| `--border-size` | `24` | Border width in pixels |
| `--bits-per-cell` | `2` | Bits per cell (1–8) |
| `--width` | `1920` | Video width in pixels |
| `--height` | `1080` | Video height in pixels |
| `--fps` | `30` | Output video framerate |
| `--crf` | `18` | H.264 quality (lower = better, 0–51) |
| `--data-shards` | `0` | RS data shards (0 = auto) |
| `--parity-shards` | `0` | RS parity shards (0 = auto) |
| `--shard-size` | `0` | Bytes per RS shard (0 = auto) |
| `--background` | — | Embed cells into an existing video (path to .mp4) |
| `--password` | — | Encrypt data with password (AES-256-GCM + Argon2id) |
| `--audio` | `false` | Also encode data into audio track |

### ystore decode

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | — | Input video file **(required)** |
| `--output` | `output.bin` | Output data file |
| `--output-dir` | — | Extract files to a directory (auto-detects archive bundles) |
| `--cell-size` | `24` | Visual grid cell size in pixels |
| `--border-size` | `24` | Border width in pixels |
| `--bits-per-cell` | `2` | Bits per cell (1–8) |
| `--width` | `1920` | Video width in pixels (must match encode) |
| `--height` | `1080` | Video height in pixels (must match encode) |
| `--fps` | `30` | Video framerate used during encode |
| `--data-shards` | `0` | RS data shards (0 = auto) |
| `--parity-shards` | `0` | RS parity shards (0 = auto) |
| `--shard-size` | `0` | Bytes per RS shard (0 = auto) |
| `--audio` | `false` | Extract data from audio instead of video |
| `--password` | — | Decrypt data with password |

> **Tip:** `ystore --version` prints the version and exits. Standalone binaries also support `--version` (e.g. `ystore-encode --version`).

---

## How It Works

### Video Channel (Primary)

```
Input File → Split into frames → RS Encode →
Pack into cell gray-levels → Render frames at configured resolution →
ffmpeg → H.264 MP4 → Upload to video platform

Download → ffmpeg extract frames → Read cell gray-levels →
RS Decode → Reassemble → Original File
```

At the default 1920×1080 resolution, each frame is a grid of 78×43 = 3354
cells (24×24 px each). With 2 bits/cell, each frame stores 838 bytes of
RS-protected data. At 30 fps the throughput is **~1.3 MB/min**.
Smaller resolutions proportionally reduce capacity.

A guard band (4 px) around each cell avoids edge artifacts from H.264
compression. Reed-Solomon corrects any remaining bit errors.

### Why it survives re-encoding

The cell grid is engineered to survive lossy compression. Each 24×24 px cell
spans multiple H.264 16×16 macroblocks, so no single macroblock decision can
erase it. Guard bands absorb edge artifacts. With only 4 gray levels (2
bits/cell), the decoder identifies the correct cell value even after luminance
drift from CRF-18 re-encoding. Reed-Solomon (~10% parity) handles the few cells
that do get corrupted.

### Audio Channel (Secondary)

```
Data → Length prefix → Pad → MFSK modulate (64 tones, 1–4 kHz) →
Generate WAV → ffmpeg → AAC → Mux with video

Extract audio → MFSK demodulate (Goertzel) → Read length → Data
```

64 evenly-spaced tones carry 6 bits/symbol. Each symbol is a 4096-sample
(93 ms) Hann-windowed sine burst. Decoder uses Goertzel's algorithm
for efficient tone detection. Throughput: **~60 bps**.

### Error Correction

Reed-Solomon parameters are auto-matched to the grid capacity. For the
default 24×24 px / 2-bit grid, the pipeline picks ~10% parity shards,
which can correct several corrupted shards per frame — enough to survive
typical H.264 compression artifacts at CRF 18.

---

## Configuration Tuning

| Parameter | Effect | Recommendation |
|-----------|--------|---------------|
| `--width` / `--height` | Larger = more cells, more data per frame | 1920×1080 for maximum capacity |
| `--cell-size` | Larger = more robust, fewer cells | 24 (default) for video platforms |
| `--bits-per-cell` | Higher = more data, less robust | 2 (default) for video platforms; 1 for extreme robustness; 3–4 for experiments |
| `--crf` | Lower = better quality, larger file | 18 (default); 23 for smaller files |

---

## Limitations

- Video platforms may change codecs (VP9, AV1). The visual grid approach survives
  because it relies on perceptual preservation, not codec specifics.
- When using `--background`, cell areas are drawn at full opacity — the
  background is only visible in non-cell regions (borders, guard bands).
- Audio channel is low-rate (~60 bps). Use it for redundancy, not primary
  transport of large payloads.
- ffmpeg must be installed separately.

---

## Project Structure

```
├── cmd/ystore/       Unified CLI (ystore encode / ystore decode)
├── cmd/encode/       Standalone encode binary (ystore-encode)
├── cmd/decode/       Standalone decode binary (ystore-decode)
├── internal/
│   ├── vcodec/       Visual grid codec (cell packing, frame rendering)
│   ├── acodec/       MFSK audio codec (tone modulation, Goertzel)
│   ├── ecc/          Reed-Solomon error correction
│   ├── formats/      Frame header structs and serialisation
│   ├── pipeline/     Encode/decode orchestration (ffmpeg exec, temp files)
│   └── version/      Version constant
├── Makefile          Build, test, lint, release
└── AGENTS.md         Context for AI coding agents
```

---

## License

[MIT](LICENSE)
