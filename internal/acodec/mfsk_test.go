package acodec

import (
	"testing"
)

func TestMFSKRoundTrip(t *testing.T) {
	cfg := DefaultMFSKConfig()

	input := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}
	t.Logf("input: %x", input)

	samples, err := EncodeMFSK(cfg, input)
	if err != nil {
		t.Fatalf("EncodeMFSK: %v", err)
	}

	t.Logf("encoded %d samples (%.2f sec)", len(samples), float64(len(samples))/SampleRate)

	output, err := DecodeMFSK(cfg, samples)
	if err != nil {
		t.Fatalf("DecodeMFSK: %v", err)
	}

	t.Logf("output: %x", output)

	if len(output) != len(input) {
		t.Fatalf("length mismatch: %d vs %d", len(output), len(input))
	}
	for i := range input {
		if output[i] != input[i] {
			t.Fatalf("byte %d: 0x%02x vs 0x%02x", i, output[i], input[i])
		}
	}
}

func TestWAVRoundTrip(t *testing.T) {
	cfg := DefaultMFSKConfig()

	input := []byte("Hello! This is test data for the MFSK audio codec.")
	t.Logf("input %d bytes: %s", len(input), input)

	wavData, err := EncodeToWAV(input, cfg)
	if err != nil {
		t.Fatalf("EncodeToWAV: %v", err)
	}
	t.Logf("WAV: %d bytes (%.2f sec)", len(wavData), float64(len(wavData)-44)/float64(SampleRate*2))

	output, err := DecodeFromWAV(wavData, cfg)
	if err != nil {
		t.Fatalf("DecodeFromWAV: %v", err)
	}

	if len(output) != len(input) {
		t.Fatalf("length: %d vs %d", len(output), len(input))
	}
	for i := range input {
		if output[i] != input[i] {
			t.Fatalf("byte %d: 0x%02x vs 0x%02x", i, output[i], input[i])
		}
	}
	t.Logf("AUDIO MFSK ROUND TRIP OK")
}
