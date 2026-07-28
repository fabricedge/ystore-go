package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/katri/ystore-go/internal/ecc"
	"github.com/katri/ystore-go/internal/formats"
	"github.com/katri/ystore-go/internal/pipeline"
	"github.com/katri/ystore-go/internal/vcodec"
)

type stringSlice []string

func (s *stringSlice) String() string {
	if len(*s) == 0 {
		return ""
	}
	return (*s)[0]
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runEncode(args []string) error {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)

	var inputs stringSlice
	fs.Var(&inputs, "input", "input file to encode (repeatable)")
	inputDir := fs.String("input-dir", "", "encode all files in a directory")
	output := fs.String("output", "output.mp4", "output video file")
	cellSize := fs.Int("cell-size", 24, "visual grid cell size in pixels")
	borderSize := fs.Int("border-size", 24, "border size in pixels")
	bitsPerCell := fs.Int("bits-per-cell", 2, "bits encoded per cell (1-8)")
	width := fs.Int("width", 1920, "video width in pixels")
	height := fs.Int("height", 1080, "video height in pixels")
	fps := fs.Int("fps", 30, "output video framerate")
	crf := fs.Int("crf", 18, "H.264 quality (lower=better, 0-51)")
	dataShards := fs.Int("data-shards", 0, "RS data shards (0=auto)")
	parityShards := fs.Int("parity-shards", 0, "RS parity shards (0=auto)")
	shardSize := fs.Int("shard-size", 0, "bytes per RS shard (0=auto)")
	audio := fs.Bool("audio", false, "also encode data into audio track")

	fs.Parse(args)

	vc := vcodec.Config{
		CellSize:    *cellSize,
		BorderSize:  *borderSize,
		BitsPerCell: *bitsPerCell,
		Width:       *width,
		Height:      *height,
	}

	if err := vc.Validate(); err != nil {
		return fmt.Errorf("invalid video config: %w", err)
	}

	var rs *ecc.ReedSolomon
	if *dataShards > 0 && *parityShards > 0 && *shardSize > 0 {
		var err error
		rs, err = ecc.NewReedSolomon(*dataShards, *parityShards, *shardSize)
		if err != nil {
			return fmt.Errorf("creating RS encoder: %w", err)
		}
		if rs.TotalBytes() > vc.MaxBytesPerFrame() {
			return fmt.Errorf("RS total (%d) exceeds grid capacity (%d)", rs.TotalBytes(), vc.MaxBytesPerFrame())
		}
	} else {
		var err error
		rs, err = pipeline.MatchRSConfig(vc)
		if err != nil {
			return err
		}
	}

	var inputFiles []string
	inputFiles = append(inputFiles, inputs...)
	if *inputDir != "" {
		entries, err := os.ReadDir(*inputDir)
		if err != nil {
			return fmt.Errorf("reading directory: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				inputFiles = append(inputFiles, filepath.Join(*inputDir, e.Name()))
			}
		}
	}
	if len(inputFiles) == 0 {
		return fmt.Errorf("no input files specified")
	}

	cfg := &pipeline.EncodeConfig{
		Video:  vc,
		RS:     rs,
		Output: *output,
		FPS:    *fps,
		CRF:    *crf,
		Audio:  *audio,
	}

	outDir := filepath.Dir(*output)
	if outDir != "" {
		os.MkdirAll(outDir, 0755)
	}

	var data []byte
	var label string

	if len(inputFiles) == 1 {
		var err error
		label = filepath.Base(inputFiles[0])
		data, err = os.ReadFile(inputFiles[0])
		if err != nil {
			return fmt.Errorf("reading %s: %w", inputFiles[0], err)
		}
	} else {
		fileInfos := make([]formats.BundledFile, 0, len(inputFiles))
		for _, path := range inputFiles {
			fd, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			fileInfos = append(fileInfos, formats.BundledFile{
				Name: filepath.Base(path),
				Data: fd,
			})
		}
		var err error
		data, err = formats.BundleFiles(fileInfos)
		if err != nil {
			return fmt.Errorf("bundling files: %w", err)
		}
		label = fmt.Sprintf("%d files", len(inputFiles))
	}

	payloadPerFrame := cfg.PayloadPerFrame()
	totalFrames := pipeline.DeduceFrameCount(cfg.Video, cfg.RS, len(data))
	duration := float64(totalFrames) / float64(cfg.FPS)

	log.Printf("Encoding: %s (%d bytes, %d frames, %.1f sec)", label, len(data), totalFrames, duration)
	log.Printf("Grid: %dx%d cells, %d bits/cell", cfg.Video.CellsPerRow(), cfg.Video.CellsPerCol(), cfg.Video.BitsPerCell)
	log.Printf("RS: %d+%d shards x %d bytes = %d total -> %d data -> %d payload/frame",
		cfg.RS.DataShards, cfg.RS.ParityShards, cfg.RS.ShardSize,
		cfg.RS.TotalBytes(), cfg.RS.DataBytes(), payloadPerFrame)
	log.Printf("Output: %s", *output)

	return pipeline.EncodeBytes(data, label, cfg)
}

func runDecode(args []string) error {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)

	input := fs.String("input", "", "input video file")
	output := fs.String("output", "output.bin", "output data file")
	outputDir := fs.String("output-dir", "", "extract archive to directory")
	cellSize := fs.Int("cell-size", 24, "visual grid cell size in pixels")
	borderSize := fs.Int("border-size", 24, "border size in pixels")
	bitsPerCell := fs.Int("bits-per-cell", 2, "bits encoded per cell (1-8)")
	width := fs.Int("width", 1920, "video width in pixels")
	height := fs.Int("height", 1080, "video height in pixels")
	fps := fs.Int("fps", 30, "video framerate used during encode")
	dataShards := fs.Int("data-shards", 0, "RS data shards (0=auto)")
	parityShards := fs.Int("parity-shards", 0, "RS parity shards (0=auto)")
	shardSize := fs.Int("shard-size", 0, "bytes per RS shard (0=auto)")
	audio := fs.Bool("audio", false, "extract data from audio track instead of video")

	fs.Parse(args)

	if *input == "" {
		return fmt.Errorf("-input is required")
	}

	vc := vcodec.Config{
		CellSize:    *cellSize,
		BorderSize:  *borderSize,
		BitsPerCell: *bitsPerCell,
		Width:       *width,
		Height:      *height,
	}

	if err := vc.Validate(); err != nil {
		return fmt.Errorf("invalid video config: %w", err)
	}

	var rs *ecc.ReedSolomon
	if *dataShards > 0 && *parityShards > 0 && *shardSize > 0 {
		var err error
		rs, err = ecc.NewReedSolomon(*dataShards, *parityShards, *shardSize)
		if err != nil {
			return fmt.Errorf("creating RS decoder: %w", err)
		}
	} else {
		var err error
		rs, err = pipeline.MatchRSConfig(vc)
		if err != nil {
			return err
		}
	}

	if *outputDir != "" {
		os.MkdirAll(*outputDir, 0755)
	}

	cfg := &pipeline.DecodeConfig{
		Video:     vc,
		RS:        rs,
		Input:     *input,
		Output:    *output,
		OutputDir: *outputDir,
		FPS:       *fps,
		Audio:     *audio,
	}

	log.Printf("Decoding: %s", *input)
	log.Printf("Grid: %dx%d cells, %d bits/cell", cfg.Video.CellsPerRow(), cfg.Video.CellsPerCol(), cfg.Video.BitsPerCell)
	log.Printf("RS: %d+%d shards x %d bytes", cfg.RS.DataShards, cfg.RS.ParityShards, cfg.RS.ShardSize)
	if *outputDir != "" {
		log.Printf("Output dir: %s", *outputDir)
	} else {
		log.Printf("Output: %s", *output)
	}

	return pipeline.DecodeFile(cfg)
}

func main() {
	log.SetFlags(0)

	if len(os.Args) >= 2 && os.Args[1] == "--version" {
		fmt.Println("ystore-go v0.3.0")
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ystore <command> [flags]\n\nCommands:\n  encode   encode data into video\n  decode   extract data from video\n")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	log.SetPrefix(cmd + ": ")

	var err error
	switch cmd {
	case "encode":
		err = runEncode(args)
	case "decode":
		err = runDecode(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}
}
