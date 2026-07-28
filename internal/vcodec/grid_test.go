package vcodec

import (
	"testing"
)

func TestPackUnpackRoundTrip(t *testing.T) {
	tests := []struct {
		bitsPerCell int
		dataSize    int
	}{
		{1, 94},
		{2, 94},
		{3, 183},
		{4, 244},
		{5, 305},
		{6, 366},
		{7, 427},
		{8, 488},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			data := make([]byte, tt.dataSize)
			for i := range data {
				data[i] = byte(i*31 + 17)
			}

			cfg := DefaultConfig()
			cfg.BitsPerCell = tt.bitsPerCell

			cellVals, err := PackBytesToCellValues(data, tt.bitsPerCell)
			if err != nil {
				t.Fatalf("PackBytesToCellValues: %v", err)
			}

			out := UnpackCellValuesToBytes(cellVals, tt.bitsPerCell)
			if len(out) != len(data) {
				t.Fatalf("round-trip length: %d vs %d", len(out), len(data))
			}
			for i := range data {
				if out[i] != data[i] {
					t.Fatalf("byte %d: 0x%02x vs 0x%02x", i, out[i], data[i])
				}
			}
		})
	}
}

func TestFrameRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	packedBytes := cfg.MaxBytesPerFrame()
	data := make([]byte, packedBytes)
	for i := range data {
		data[i] = byte(i*7 + 11)
	}

	t.Logf("bitsPerCell=%d, cells=%d, usedCells=%d, maxBytes=%d",
		cfg.BitsPerCell, cfg.TotalCells(), cfg.UsedCells(), packedBytes)

	img, err := EncodeFrame(cfg, data)
	if err != nil {
		t.Fatalf("EncodeFrame: %v", err)
	}
	if img == nil {
		t.Fatal("EncodeFrame returned nil image")
	}

	out, err := DecodeFrame(cfg, img)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	if len(out) != len(data) {
		t.Fatalf("length: %d vs %d", len(out), len(data))
	}

	mismatches := 0
	for i := range data {
		if out[i] != data[i] {
			if mismatches < 10 {
				t.Logf("byte %d: expected 0x%02x, got 0x%02x", i, data[i], out[i])
			}
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Fatalf("total mismatches: %d / %d", mismatches, len(data))
	}
}
