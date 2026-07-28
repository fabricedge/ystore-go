package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/katri/ystore-go/internal/vcodec"
)

func TestEncodeDecodeCycle(t *testing.T) {
	ffmpegPath, err := findFFmpeg()
	if err != nil {
		t.Skip("ffmpeg not available:", err)
	}
	t.Logf("using ffmpeg: %s", ffmpegPath)

	tmpDir, err := os.MkdirTemp("", "ystore-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.bin")
	videoPath := filepath.Join(tmpDir, "output.mp4")
	outputPath := filepath.Join(tmpDir, "decoded.bin")

	vc := vcodec.DefaultConfig()
	rs, err := MatchRSConfig(vc)
	if err != nil {
		t.Fatal("matching RS config:", err)
	}

	encCfg := &EncodeConfig{
		Video:  vc,
		RS:     rs,
		Output: videoPath,
		FPS:    30,
		CRF:    18,
		Audio:  false,
	}

	payloadPerFrame := encCfg.PayloadPerFrame()
	t.Logf("grid: %dx%d cells, %d bits/cell → %d bytes/frame",
		vc.CellsPerRow(), vc.CellsPerCol(), vc.BitsPerCell, vc.MaxBytesPerFrame())
	t.Logf("RS: %d+%d shards × %d bytes = %d total → %d data → %d payload/frame",
		rs.DataShards, rs.ParityShards, rs.ShardSize, rs.TotalBytes(),
		rs.DataBytes(), payloadPerFrame)

	inputData := make([]byte, 15000)
	for i := range inputData {
		inputData[i] = byte(i*17 + 53)
	}

	frames := (len(inputData) + payloadPerFrame - 1) / payloadPerFrame
	t.Logf("input: %d bytes → %d frames at %d fps = %.1f sec",
		len(inputData), frames, encCfg.FPS, float64(frames)/float64(encCfg.FPS))

	if err := os.WriteFile(inputPath, inputData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := EncodeFile(inputPath, encCfg); err != nil {
		t.Fatal("encode:", err)
	}

	stat, err := os.Stat(videoPath)
	if err != nil {
		t.Fatal("video not created:", err)
	}
	t.Logf("video: %d bytes", stat.Size())

	decCfg := &DecodeConfig{
		Video:  vc,
		RS:     rs,
		Input:  videoPath,
		Output: outputPath,
		FPS:    30,
		Audio:  false,
	}

	if err := DecodeFile(decCfg); err != nil {
		t.Logf("decode error: %v", err)
	}

	decoded, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal("reading decoded:", err)
	}

	t.Logf("decoded: %d bytes, expected: %d", len(decoded), len(inputData))

	if len(decoded) != len(inputData) {
		minLen := len(decoded)
		if len(inputData) < minLen {
			minLen = len(inputData)
		}
		diffs := 0
		for i := 0; i < minLen; i++ {
			if decoded[i] != inputData[i] {
				if diffs < 5 {
					t.Logf("  byte %d: 0x%02x vs 0x%02x", i, decoded[i], inputData[i])
				}
				diffs++
			}
		}
		t.Logf("total diff bytes: %d / %d", diffs, minLen)
		t.Fatalf("size mismatch: %d vs %d", len(decoded), len(inputData))
	}

	for i := range inputData {
		if decoded[i] != inputData[i] {
			t.Fatalf("byte %d mismatch: 0x%02x vs 0x%02x", i, decoded[i], inputData[i])
		}
	}

	t.Logf("SUCCESS: %d bytes encoded/decoded correctly", len(inputData))
}
