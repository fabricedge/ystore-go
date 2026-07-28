package pipeline

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/katri/ystore-go/internal/acodec"
	"github.com/katri/ystore-go/internal/crypto"
	"github.com/katri/ystore-go/internal/ecc"
	"github.com/katri/ystore-go/internal/formats"
	"github.com/katri/ystore-go/internal/vcodec"
)

type DecodeConfig struct {
	Video     vcodec.Config
	RS        *ecc.ReedSolomon
	Input     string
	Output    string
	OutputDir string
	FPS       int
	Audio     bool
	Password  string
}

func DefaultDecodeConfig() (*DecodeConfig, error) {
	rs, err := ecc.NewReedSolomon(200, 50, 9)
	if err != nil {
		return nil, err
	}
	return &DecodeConfig{
		Video:  vcodec.DefaultConfig(),
		RS:     rs,
		Input:  "input.mp4",
		Output: "output.bin",
		FPS:    30,
		Audio:  false,
	}, nil
}

func DecodeFile(cfg *DecodeConfig) error {
	ffmpegPath, err := findFFmpeg()
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ystore-decode-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractFrames(ffmpegPath, cfg.Input, tmpDir, cfg.FPS); err != nil {
		return fmt.Errorf("extracting frames: %w", err)
	}

	frameFiles, err := GetFrameFiles(tmpDir, 0)
	if err != nil {
		return fmt.Errorf("listing frames: %w", err)
	}

	if len(frameFiles) == 0 {
		return fmt.Errorf("no frames extracted from video")
	}

	frameResults := make([]struct {
		num  uint32
		data []byte
	}, 0, len(frameFiles))

	for _, framePath := range frameFiles {
		fh, err := os.Open(framePath)
		if err != nil {
			return fmt.Errorf("opening frame %s: %w", framePath, err)
		}

		img, err := png.Decode(fh)
		fh.Close()
		if err != nil {
			return fmt.Errorf("decoding PNG %s: %w", framePath, err)
		}

		gridData, err := vcodec.DecodeFrame(cfg.Video, img)
		if err != nil {
			return fmt.Errorf("decoding frame %s: %w", framePath, err)
		}

		rsLen := cfg.RS.TotalBytes()
		if len(gridData) < rsLen {
			continue
		}

		rsDecoded, err := cfg.RS.Decode(gridData[:rsLen])
		if err != nil {
			return fmt.Errorf("RS decode frame %s: %w", framePath, err)
		}

		if len(rsDecoded) < formats.FrameHeaderSize*2 {
			continue
		}

		hdr, totalFrames, dataLen, err := formats.UnmarshalFrameFull(rsDecoded[:formats.FrameHeaderSize*2])
		if err != nil {
			continue
		}

		if hdr.Magic != formats.FrameMagic {
			continue
		}

		if int(dataLen) > len(rsDecoded)-formats.FrameHeaderSize*2 {
			dataLen = uint32(len(rsDecoded) - formats.FrameHeaderSize*2)
		}

		payload := make([]byte, dataLen)
		copy(payload, rsDecoded[formats.FrameHeaderSize*2:formats.FrameHeaderSize*2+dataLen])

		frameResults = append(frameResults, struct {
			num  uint32
			data []byte
		}{num: hdr.FrameNumber, data: payload})

		if len(frameResults) >= int(totalFrames) {
			break
		}
	}

	if len(frameResults) == 0 {
		return fmt.Errorf("no valid frames found in video")
	}

	for i := 0; i < len(frameResults); i++ {
		for j := i + 1; j < len(frameResults); j++ {
			if frameResults[j].num < frameResults[i].num {
				frameResults[i], frameResults[j] = frameResults[j], frameResults[i]
			}
		}
	}

	var reconstructed []byte
	seen := make(map[uint32]bool)
	for _, fr := range frameResults {
		if seen[fr.num] {
			continue
		}
		seen[fr.num] = true
		reconstructed = append(reconstructed, fr.data...)
	}

	if cfg.Audio {
		audioData, err := extractAudio(ffmpegPath, cfg.Input)
		if err == nil {
			audioCfg := acodec.DefaultMFSKConfig()
			audioDecoded, err := acodec.DecodeFromWAV(audioData, audioCfg)
			if err == nil && len(audioDecoded) > 0 {
				reconstructed = audioDecoded
			}
		}
	}

	if cfg.Password != "" {
		var err error
		reconstructed, err = crypto.Decrypt(reconstructed, cfg.Password)
		if err != nil {
			return fmt.Errorf("decrypt: %w", err)
		}
	}

	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			return fmt.Errorf("creating output dir: %w", err)
		}

		if formats.IsArchive(reconstructed) {
			files, err := formats.ExtractFiles(reconstructed)
			if err != nil {
				return fmt.Errorf("extracting archive: %w", err)
			}
			for _, f := range files {
				outPath := filepath.Join(cfg.OutputDir, f.Name)
				if err := os.WriteFile(outPath, f.Data, 0644); err != nil {
					return fmt.Errorf("writing %s: %w", outPath, err)
				}
				fmt.Printf("  extracted: %s (%d bytes)\n", f.Name, len(f.Data))
			}
		} else {
			outPath := filepath.Join(cfg.OutputDir, "output.bin")
			if err := os.WriteFile(outPath, reconstructed, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
		}
	} else {
		if err := os.WriteFile(cfg.Output, reconstructed, 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	return nil
}

func extractFrames(ffmpegPath, videoPath, outputDir string, fps int) error {
	outputPattern := filepath.Join(outputDir, "frame_%05d.png")

	args := []string{
		"-y",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%d", fps),
		"-pix_fmt", "rgba",
		outputPattern,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract frames failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func extractAudio(ffmpegPath, videoPath string) ([]byte, error) {
	outputPath := videoPath + ".extracted.wav"
	defer os.Remove(outputPath)

	args := []string{
		"-y",
		"-i", videoPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", fmt.Sprintf("%d", acodec.SampleRate),
		"-ac", "1",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg extract audio failed: %w\nstderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("reading extracted audio: %w", err)
	}
	return data, nil
}

func parseFrameNumber(filename string) (int, error) {
	base := filepath.Base(filename)
	base = strings.TrimPrefix(base, "frame_")
	base = strings.TrimSuffix(base, ".png")
	return strconv.Atoi(base)
}
