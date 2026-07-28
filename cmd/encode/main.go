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

func main() {
	log.SetFlags(0)
	log.SetPrefix("encode: ")

	var inputs stringSlice
	flag.Var(&inputs, "input", "input file to encode (repeatable: --input a.pdf --input b.pdf)")
	inputDir := flag.String("input-dir", "", "encode all files in a directory")
	output := flag.String("output", "output.mp4", "output video file")

	cellSize := flag.Int("cell-size", 24, "visual grid cell size in pixels")
	borderSize := flag.Int("border-size", 24, "border size in pixels")
	bitsPerCell := flag.Int("bits-per-cell", 2, "bits encoded per cell (1-8)")
	fps := flag.Int("fps", 30, "output video framerate")
	crf := flag.Int("crf", 18, "H.264 quality (lower=better, 0-51)")

	dataShards := flag.Int("data-shards", 0, "Reed-Solomon data shards (0=auto)")
	parityShards := flag.Int("parity-shards", 0, "Reed-Solomon parity shards (0=auto)")
	shardSize := flag.Int("shard-size", 0, "bytes per RS shard (0=auto)")
	audio := flag.Bool("audio", false, "also encode data into audio track")

	showVersion := flag.Bool("version", false, "show version")

	flag.Parse()

	if *showVersion {
		fmt.Println("ystore-go encode v0.1.0")
		return
	}

	if len(inputs) == 0 && *inputDir == "" {
		log.Fatal("either --input or --input-dir is required")
	}

	outDir := filepath.Dir(*output)
	if outDir != "" {
		os.MkdirAll(outDir, 0755)
	}

	vc := vcodec.Config{
		CellSize:    *cellSize,
		BorderSize:  *borderSize,
		BitsPerCell: *bitsPerCell,
	}

	if err := vc.Validate(); err != nil {
		log.Fatalf("invalid video config: %v", err)
	}

	var rs *ecc.ReedSolomon
	if *dataShards > 0 && *parityShards > 0 && *shardSize > 0 {
		var err error
		rs, err = ecc.NewReedSolomon(*dataShards, *parityShards, *shardSize)
		if err != nil {
			log.Fatalf("creating RS encoder: %v", err)
		}
		if rs.TotalBytes() > vc.MaxBytesPerFrame() {
			log.Fatalf("RS total (%d) exceeds grid capacity (%d); reduce shard count/size",
				rs.TotalBytes(), vc.MaxBytesPerFrame())
		}
	} else {
		var err error
		rs, err = pipeline.MatchRSConfig(vc)
		if err != nil {
			log.Fatal(err)
		}
	}

	var inputFiles []string
	inputFiles = append(inputFiles, inputs...)

	if *inputDir != "" {
		entries, err := os.ReadDir(*inputDir)
		if err != nil {
			log.Fatalf("reading directory %s: %v", *inputDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				inputFiles = append(inputFiles, filepath.Join(*inputDir, e.Name()))
			}
		}
		if len(inputFiles) == 0 {
			log.Fatalf("no files found in directory %s", *inputDir)
		}
	}

	if len(inputFiles) == 0 {
		log.Fatal("no input files specified")
	}

	cfg := &pipeline.EncodeConfig{
		Video:  vc,
		RS:     rs,
		Output: *output,
		FPS:    *fps,
		CRF:    *crf,
		Audio:  *audio,
	}

	payloadPerFrame := cfg.PayloadPerFrame()
	if payloadPerFrame <= 0 {
		log.Fatal("payload per frame is zero or negative")
	}

	var label string
	var data []byte

	if len(inputFiles) == 1 {
		var err error
		label = filepath.Base(inputFiles[0])
		data, err = os.ReadFile(inputFiles[0])
		if err != nil {
			log.Fatalf("reading %s: %v", inputFiles[0], err)
		}
		totalFrames := pipeline.DeduceFrameCount(cfg.Video, cfg.RS, len(data))
		duration := float64(totalFrames) / float64(cfg.FPS)
		log.Printf("Encoding: %s (%d bytes, %d frames, %.1f sec)", label, len(data), totalFrames, duration)
	} else {
		fileInfos := make([]formats.BundledFile, 0, len(inputFiles))
		for _, path := range inputFiles {
			fi, err := os.Stat(path)
			if err != nil {
				log.Fatalf("stat %s: %v", path, err)
			}
			fileData, err := os.ReadFile(path)
			if err != nil {
				log.Fatalf("reading %s: %v", path, err)
			}
			fileInfos = append(fileInfos, formats.BundledFile{
				Name: filepath.Base(path),
				Data: fileData,
			})
			log.Printf("  input: %s (%d bytes)", filepath.Base(path), fi.Size())
		}

		var err error
		data, err = formats.BundleFiles(fileInfos)
		if err != nil {
			log.Fatalf("bundling files: %v", err)
		}

		label = fmt.Sprintf("%d files", len(inputFiles))
		totalFrames := pipeline.DeduceFrameCount(cfg.Video, cfg.RS, len(data))
		duration := float64(totalFrames) / float64(cfg.FPS)
		log.Printf("Encoding: %s bundle (%d bytes, %d frames, %.1f sec)", label, len(data), totalFrames, duration)
	}

	log.Printf("Grid: %dx%d cells, %d bits/cell", cfg.Video.CellsPerRow(), cfg.Video.CellsPerCol(), cfg.Video.BitsPerCell)
	log.Printf("RS: %d+%d shards x %d bytes = %d total -> %d data -> %d payload/frame",
		cfg.RS.DataShards, cfg.RS.ParityShards, cfg.RS.ShardSize,
		cfg.RS.TotalBytes(), cfg.RS.DataBytes(), payloadPerFrame)
	log.Printf("Output: %s", *output)

	if err := pipeline.EncodeBytes(data, label, cfg); err != nil {
		log.Fatalf("encode failed: %v", err)
	}

	log.Println("Done")
}
