# DTMF tone generation

Generates DTMF (dual-tone multi-frequency) WAV files for keys 0–9, `*`, and `#`, using ITU-T Q.23 frequencies.

## Requirements

- **ffmpeg** (must be on `PATH`)

## Usage

From this directory:

```sh
./generate_dtmf.sh
```

Output: `dtmf-0.wav` … `dtmf-9.wav`, `dtmf-star.wav`, `dtmf-pound.wav` (8 kHz, mono, 16-bit PCM; tone + gap).

From repo root:

```sh
./scripts/dtmf/generate_dtmf.sh
```

## Optional: Go generator (no ffmpeg)

From repo root, generate WAVs with Go only:

```sh
go run ./cmd/generate-dtmf [output_dir]
```

Default output dir is current directory. Uses `pkg/audio` (DTMF frequencies and WAV writing).

## Relation to codebase

DTMF keys and frames are defined in `pkg/frames/dtmf.go` (`KeypadEntry`, `OutputDTMFUrgentFrame`). These WAVs are useful for tests or playback when implementing IVR/telephony.
