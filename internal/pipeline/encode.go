package pipeline

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/katri/ystore-go/internal/acodec"
	"github.com/katri/ystore-go/internal/crypto"
	"github.com/katri/ystore-go/internal/ecc"
	"github.com/katri/ystore-go/internal/formats"
	"github.com/katri/ystore-go/internal/vcodec"
)

type EncodeConfig struct {
	Video       vcodec.Config
	RS          *ecc.ReedSolomon
	Input       string
	Inputs      []string
	Output      string
	FPS         int
	CRF         int
	Audio       bool
	Background  string
	Password    string
	MinDuration float64
}

func DefaultEncodeConfig() (*EncodeConfig, error) {
	rs, err := MatchRSConfig(vcodec.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return &EncodeConfig{
		Video:  vcodec.DefaultConfig(),
		RS:     rs,
		Output: "output.mp4",
		FPS:    30,
		CRF:    18,
		Audio:  false,
	}, nil
}

func MatchRSConfig(vc vcodec.Config) (*ecc.ReedSolomon, error) {
	totalBytes := vc.MaxBytesPerFrame()

	type candidate struct {
		rs          *ecc.ReedSolomon
		waste       int
		parityRatio float64
	}

	var best *candidate
	maxShards := totalBytes
	if maxShards > 255 {
		maxShards = 255
	}

	for totalShards := 3; totalShards <= maxShards; totalShards++ {
		shardSize := totalBytes / totalShards
		actualTotal := totalShards * shardSize
		waste := totalBytes - actualTotal

		if waste > 16 || shardSize < 2 {
			continue
		}

		minData := totalShards * 7 / 10
		if minData < 1 {
			minData = 1
		}
		maxData := totalShards * 9 / 10
		if maxData >= totalShards {
			maxData = totalShards - 1
		}

		for dataShards := minData; dataShards <= maxData; dataShards++ {
			parityShards := totalShards - dataShards
			if parityShards < 1 {
				continue
			}

			rs, err := ecc.NewReedSolomon(dataShards, parityShards, shardSize)
			if err != nil {
				continue
			}
			if rs.TotalBytes() != actualTotal {
				continue
			}

			ratio := float64(parityShards) / float64(dataShards)
			if ratio < 0.08 || ratio > 0.45 {
				continue
			}

			if best == nil || waste < best.waste || (waste == best.waste && ratio < best.parityRatio) {
				best = &candidate{rs: rs, waste: waste, parityRatio: ratio}
			}
		}
	}

	if best == nil {
		for totalShards := 2; totalShards <= maxShards; totalShards++ {
			shardSize := totalBytes / totalShards
			actualTotal := totalShards * shardSize
			waste := totalBytes - actualTotal
			if waste > totalBytes/4 || shardSize < 1 {
				continue
			}

			for dataShards := totalShards/4 + 1; dataShards < totalShards; dataShards++ {
				parityShards := totalShards - dataShards
				if parityShards < 1 || parityShards > totalShards*3/4 {
					continue
				}
				rs, err := ecc.NewReedSolomon(dataShards, parityShards, shardSize)
				if err != nil {
					continue
				}
				if rs.TotalBytes() != actualTotal {
					continue
				}

				if best == nil || waste < best.waste {
					best = &candidate{rs: rs, waste: waste, parityRatio: float64(parityShards) / float64(dataShards)}
				}
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("cannot match RS params to grid capacity %d bytes", totalBytes)
	}

	return best.rs, nil
}

func (c *EncodeConfig) PayloadPerFrame() int {
	headerBytes := formats.FrameHeaderSize * 2
	dataBytes := c.RS.DataBytes()
	return dataBytes - headerBytes
}

func EncodeFile(inputPath string, cfg *EncodeConfig) error {
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	return EncodeBytes(inputData, inputPath, cfg)
}

func EncodeBytes(data []byte, label string, cfg *EncodeConfig) error {
	if cfg.Password != "" {
		var err error
		data, err = crypto.Encrypt(data, cfg.Password)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}
	}

	ffmpegPath, err := findFFmpeg()
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("input data is empty")
	}

	if err := cfg.Video.Validate(); err != nil {
		return fmt.Errorf("invalid video config: %w", err)
	}

	rsTotal := cfg.RS.TotalBytes()
	gridBytes := cfg.Video.MaxBytesPerFrame()
	if rsTotal > gridBytes {
		return fmt.Errorf("RS total bytes (%d) exceeds grid capacity (%d)", rsTotal, gridBytes)
	}

	tmpDir, err := os.MkdirTemp("", "ystore-encode-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	payloadPerFrame := cfg.PayloadPerFrame()
	totalFrames := int((len(data) + payloadPerFrame - 1) / payloadPerFrame)
	if totalFrames < 1 {
		totalFrames = 1
	}

	if cfg.MinDuration > 0 {
		minFrames := int(math.Ceil(cfg.MinDuration * float64(cfg.FPS)))
		if minFrames > totalFrames {
			totalFrames = minFrames
		}
	}

	var bgDir string
	var bgFrameCount int
	if cfg.Background != "" {
		bgDir, err = os.MkdirTemp("", "ystore-bg-*")
		if err != nil {
			return fmt.Errorf("creating bg temp dir: %w", err)
		}
		defer os.RemoveAll(bgDir)

		if err := extractFrames(ffmpegPath, cfg.Background, bgDir, cfg.FPS); err != nil {
			return fmt.Errorf("extracting background frames: %w", err)
		}

		bgFrames, err := GetFrameFiles(bgDir, 0)
		if err != nil {
			return fmt.Errorf("listing background frames: %w", err)
		}
		bgFrameCount = len(bgFrames)
		if bgFrameCount == 0 {
			return fmt.Errorf("no frames extracted from background video")
		}
	}

	headerBuf := make([]byte, formats.FrameHeaderSize*2)

	for f := 0; f < totalFrames; f++ {
		var chunk []byte
		start := f * payloadPerFrame
		if start < len(data) {
			end := start + payloadPerFrame
			if end > len(data) {
				end = len(data)
			}
			chunk = data[start:end]
		}

		fh := formats.NewFrameHeader(uint32(f), uint32(totalFrames), uint32(len(chunk)))
		fullHeader := fh.MarshalFull(uint32(totalFrames), uint32(len(chunk)))
		copy(headerBuf, fullHeader)

		frameData := make([]byte, cfg.RS.DataBytes())
		copy(frameData, headerBuf)
		if len(chunk) > 0 {
			copy(frameData[formats.FrameHeaderSize*2:], chunk)
		}

		encoded, err := cfg.RS.Encode(frameData)
		if err != nil {
			return fmt.Errorf("RS encode frame %d: %w", f, err)
		}

		if len(encoded) < gridBytes {
			padded := make([]byte, gridBytes)
			copy(padded, encoded)
			encoded = padded
		}

		if cfg.Background != "" && bgFrameCount > 0 {
			bgIdx := f % bgFrameCount
			bgFrameList, _ := GetFrameFiles(bgDir, bgFrameCount)
			bgPath := bgFrameList[bgIdx]

			bgFH, err := os.Open(bgPath)
			if err != nil {
				return fmt.Errorf("opening bg frame %s: %w", bgPath, err)
			}
			bgImg, err := png.Decode(bgFH)
			bgFH.Close()
			if err != nil {
				return fmt.Errorf("decoding bg frame: %w", err)
			}

			bgRGBA := image.NewRGBA(bgImg.Bounds())
			for y := 0; y < bgRGBA.Bounds().Dy(); y++ {
				for x := 0; x < bgRGBA.Bounds().Dx(); x++ {
					bgRGBA.Set(x, y, bgImg.At(x, y))
				}
			}

			img, err := vcodec.EncodeFrameBlend(cfg.Video, encoded, bgRGBA)
			if err != nil {
				return fmt.Errorf("encode blended frame %d: %w", f, err)
			}

			framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%05d.png", f))
			fhOut, err := os.Create(framePath)
			if err != nil {
				return fmt.Errorf("creating frame file: %w", err)
			}
			if err := png.Encode(fhOut, img); err != nil {
				fhOut.Close()
				return fmt.Errorf("encoding PNG: %w", err)
			}
			fhOut.Close()
		} else {
			img, err := vcodec.EncodeFrame(cfg.Video, encoded)
			if err != nil {
				return fmt.Errorf("encode frame %d: %w", f, err)
			}

			framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%05d.png", f))
			fhOut, err := os.Create(framePath)
			if err != nil {
				return fmt.Errorf("creating frame file: %w", err)
			}
			if err := png.Encode(fhOut, img); err != nil {
				fhOut.Close()
				return fmt.Errorf("encoding PNG: %w", err)
			}
			fhOut.Close()
		}
	}

	if cfg.Audio {
		audioPath := filepath.Join(tmpDir, "audio.wav")
		audioCfg := acodec.DefaultMFSKConfig()
		audioData, err := acodec.EncodeToWAV(data, audioCfg)
		if err != nil {
			return fmt.Errorf("encoding audio: %w", err)
		}
		if err := os.WriteFile(audioPath, audioData, 0644); err != nil {
			return fmt.Errorf("writing audio: %w", err)
		}

		videoPath := filepath.Join(tmpDir, "video.mp4")
		if err := createVideo(ffmpegPath, tmpDir, totalFrames, cfg.FPS, cfg.CRF, videoPath); err != nil {
			return fmt.Errorf("creating video: %w", err)
		}

		if err := muxAudioVideo(ffmpegPath, videoPath, audioPath, cfg.Output); err != nil {
			return fmt.Errorf("muxing audio: %w", err)
		}
	} else {
		if err := createVideo(ffmpegPath, tmpDir, totalFrames, cfg.FPS, cfg.CRF, cfg.Output); err != nil {
			return fmt.Errorf("creating video: %w", err)
		}
	}

	return nil
}

func ResolveInputs(cfg *EncodeConfig) (string, []byte, error) {
	sources := cfg.Inputs
	if len(sources) == 0 && cfg.Input != "" {
		sources = []string{cfg.Input}
	}
	if len(sources) == 0 {
		return "", nil, fmt.Errorf("no input files specified")
	}

	if len(sources) == 1 {
		data, err := os.ReadFile(sources[0])
		if err != nil {
			return "", nil, fmt.Errorf("reading %s: %w", sources[0], err)
		}
		return filepath.Base(sources[0]), data, nil
	}

	files, err := formats.ReadFilesFromDisk(sources)
	if err != nil {
		return "", nil, err
	}

	bundle, err := formats.BundleFiles(files)
	if err != nil {
		return "", nil, fmt.Errorf("bundling files: %w", err)
	}

	label := fmt.Sprintf("%d files", len(sources))
	return label, bundle, nil
}

func createVideo(ffmpegPath, frameDir string, numFrames, fps, crf int, outputPath string) error {
	firstFrame := filepath.Join(frameDir, "frame_%05d.png")

	args := []string{
		"-y",
		"-framerate", fmt.Sprintf("%d", fps),
		"-i", firstFrame,
		"-c:v", "libx264",
		"-profile:v", "high",
		"-crf", fmt.Sprintf("%d", crf),
		"-pix_fmt", "yuv420p",
		"-preset", "medium",
		"-r", fmt.Sprintf("%d", fps),
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg encode failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func muxAudioVideo(ffmpegPath, videoPath, audioPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mux failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func findFFmpeg() (string, error) {
	paths := []string{
		"ffmpeg",
		"/usr/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
		"/tmp/opencode/ffmpeg/ffmpeg",
	}

	extra := os.Getenv("PATH")
	for _, p := range strings.Split(extra, ":") {
		if p != "" {
			candidate := filepath.Join(p, "ffmpeg")
			paths = append(paths, candidate)
		}
	}

	seen := make(map[string]bool)
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("ffmpeg not found in PATH or common locations")
}

func GetFrameFiles(dir string, totalFrames int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var frames []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "frame_") && strings.HasSuffix(e.Name(), ".png") {
			frames = append(frames, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(frames)

	if totalFrames > 0 && len(frames) < totalFrames {
		return frames, fmt.Errorf("expected %d frames, got %d", totalFrames, len(frames))
	}

	if totalFrames > 0 && len(frames) > totalFrames {
		return frames[:totalFrames], nil
	}

	return frames, nil
}

func DeduceFrameCount(vcodecCfg vcodec.Config, rs *ecc.ReedSolomon, fileSize int) int {
	headerBytes := formats.FrameHeaderSize * 2
	dataBytes := rs.DataBytes()
	payloadPerFrame := dataBytes - headerBytes
	if payloadPerFrame <= 0 {
		return 0
	}
	return (fileSize + payloadPerFrame - 1) / payloadPerFrame
}
