# ystore-go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/katri/ystore-go)](https://goreportcard.com/report/github.com/katri/ystore-go)

**Hide arbitrary data inside 1920×1080 H.264 video files (and their AAC audio
track) in a way that survives YouTube re-encoding.**

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
| `ystore-encode-linux-amd64` / `ystore-decode-linux-amd64` | Linux x86_64 |
| `ystore-encode-linux-arm64` / `ystore-decode-linux-arm64` | Linux ARM64 |
| `ystore-encode-darwin-amd64` / `ystore-decode-darwin-amd64` | macOS Intel |
| `ystore-encode-darwin-arm64` / `ystore-decode-darwin-arm64` | macOS Apple Silicon |
| `ystore-encode-windows-amd64.exe` / `ystore-decode-windows-amd64.exe` | Windows x86_64 |

**Prerequisite:** [ffmpeg](https://ffmpeg.org/) must be installed and in `PATH`.

---

## Usage

### Encode a single file into a video

```bash
ystore-encode --input secret.pdf --output video.mp4
```

This produces a 1920×1080 H.264 MP4. Upload it to YouTube, share it, store it
anywhere. The data survives re-encoding.

### Encode multiple files (bundled)

```bash
ystore-encode --input doc1.pdf --input doc2.pdf --input doc3.jpg --output bundle.mp4
```

Multiple files are automatically bundled into a tar archive with a magic prefix.
Decoding auto-detects the bundle.

### Encode all files from a directory

```bash
ystore-encode --input-dir ./documents --output bundle.mp4
```

### Decode a video back to the original file

```bash
ystore-decode --input video.mp4 --output restored.pdf
```

### Decode a multi-file bundle to a directory

```bash
ystore-decode --input bundle.mp4 --output-dir ./extracted
```

If the video contains a bundled archive, files are extracted to the given
directory. Single files are written as `output.bin`.

### With audio redundancy

```bash
ystore-encode --input secret.pdf --output video.mp4 --audio
ystore-decode --input video.mp4 --output restored.pdf --audio
```

The audio channel carries the same data at a lower rate and can be used as a
backup if the video track is damaged.

---

## CLI Reference

### ystore-encode

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | — | Input file to encode (repeatable: `--input a.pdf --input b.pdf`) |
| `--input-dir` | — | Encode all files in a directory |
| `--output` | `output.mp4` | Output video file |
| `--cell-size` | `24` | Visual grid cell size in pixels |
| `--border-size` | `24` | Border width in pixels |
| `--bits-per-cell` | `2` | Bits per cell (1–8) |
| `--fps` | `30` | Output video framerate |
| `--crf` | `18` | H.264 quality (lower = better, 0–51) |
| `--data-shards` | `0` | RS data shards (0 = auto) |
| `--parity-shards` | `0` | RS parity shards (0 = auto) |
| `--shard-size` | `0` | Bytes per RS shard (0 = auto) |
| `--audio` | `false` | Also encode data into audio track |
| `--version` | — | Print version and exit |

### ystore-decode

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | — | Input video file **(required)** |
| `--output` | `output.bin` | Output data file |
| `--output-dir` | — | Extract files to a directory (auto-detects archive bundles) |
| `--cell-size` | `24` | Visual grid cell size in pixels |
| `--border-size` | `24` | Border width in pixels |
| `--bits-per-cell` | `2` | Bits per cell (1–8) |
| `--fps` | `30` | Vide framerate used during encode |
| `--data-shards` | `0` | RS data shards (0 = auto) |
| `--parity-shards` | `0` | RS parity shards (0 = auto) |
| `--shard-size` | `0` | Bytes per RS shard (0 = auto) |
| `--audio` | `false` | Extract data from audio instead of video |
| `--version` | — | Print version and exit |

---

## How It Works

### Video Channel (Primary)

```
Input File → [Encrypt] → Split into frames → RS Encode →
Pack into cell gray-levels → Render 1920×1080 frames →
ffmpeg → H.264 MP4 → Upload to YouTube

Download → ffmpeg extract frames → Read cell gray-levels →
RS Decode → Reassemble → [Decrypt] → Original File
```

Each frame is a grid of 78×43 = 3354 cells (24×24 px each).
With 2 bits/cell, each frame stores 838 bytes of RS-protected data.
At 30 fps the throughput is **~1.3 MB/min**.

A guard band (4 px) around each cell avoids edge artifacts from H.264
compression. Reed-Solomon corrects any remaining bit errors.

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
| `--cell-size` | Larger = more robust, fewer cells | 24 (default) for YouTube |
| `--bits-per-cell` | Higher = more data, less robust | 2 (default) for YouTube; 1 for extreme robustness; 3–4 for experiments |
| `--crf` | Lower = better quality, larger file | 18 (default); 23 for smaller files |

---

## Limitations

- YouTube may change codecs (VP9, AV1). The visual grid approach survives
  because it relies on perceptual preservation, not codec specifics.
- High-motion scenes may blur grid cells. Mitigation: use static/solid
  backgrounds in the video.
- Audio channel is low-rate (~60 bps). Use it for redundancy, not primary
  transport of large payloads.
- ffmpeg must be installed separately.

---

## Project Structure

```
├── cmd/encode/       CLI entry point for encoding data into video
├── cmd/decode/       CLI entry point for extracting data from video
├── internal/
│   ├── vcodec/       Visual grid codec (cell packing, frame rendering)
│   ├── acodec/       MFSK audio codec (tone modulation, Goertzel)
│   ├── ecc/          Reed-Solomon error correction
│   ├── formats/      Frame header structs and serialisation
│   └── pipeline/     Encode/decode orchestration (ffmpeg exec, temp files)
├── Makefile          Build, test, lint, release
└── AGENTS.md         Context for AI coding agents
```

---

## License

[MIT](LICENSE)
