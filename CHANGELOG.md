# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-07-27

### Added

- Configurable video resolution (`--width` / `--height` flags on encode and decode).
  Defaults to 1920×1080; any size ≥64×64 supported.
- RS auto-match now adapts to any resolution.

### Changed

- Unified `ystore` binary with `ystore encode` / `ystore decode` subcommands.
- Multi-file bundling via repeatable `--input` and `--input-dir`.
- `--output-dir` flag on decode extracts archive bundles to a directory.
- ffmpeg output written directly to final path (fixes cross-device rename error).

## [Unreleased]

### Added

- Initial project structure with `cmd/encode` and `cmd/decode` CLI tools.
- Visual grid codec (`internal/vcodec`): encodes data as gray-level cells
  in 1920×1080 video frames, survives H.264 re-encoding.
- MFSK audio codec (`internal/acodec`): OFDM-like multi-tone modulation,
  Goertzel detection, WAV I/O. Survives AAC compression.
- Reed-Solomon error correction (`internal/ecc`): auto-matched to grid
  capacity for per-frame protection.
- Pipeline orchestration (`internal/pipeline`): temp file management,
  ffmpeg frame extraction/encoding, frame sorting, data reassembly.
- Frame header format (`internal/formats`): magic bytes, frame number,
  total frames, data length.
- Makefile with `build`, `test`, `lint`, `clean`, `release` targets.
- GitHub Actions CI (`test.yml`, `release.yml`).
- AGENTS.md, CHANGELOG.md, CONTRIBUTING.md, LICENSE, README.md.
