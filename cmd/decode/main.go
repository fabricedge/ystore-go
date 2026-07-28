package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/katri/ystore-go/internal/ecc"
	"github.com/katri/ystore-go/internal/pipeline"
	"github.com/katri/ystore-go/internal/version"
	"github.com/katri/ystore-go/internal/vcodec"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("decode: ")

	input := flag.String("input", "", "input video file")
	output := flag.String("output", "output.bin", "output data file")
	outputDir := flag.String("output-dir", "", "extract files to directory (auto-detects archive bundles)")

	cellSize := flag.Int("cell-size", 24, "visual grid cell size in pixels")
	borderSize := flag.Int("border-size", 24, "border size in pixels")
	bitsPerCell := flag.Int("bits-per-cell", 2, "bits encoded per cell (1-8)")
	width := flag.Int("width", 1920, "video width in pixels")
	height := flag.Int("height", 1080, "video height in pixels")
	fps := flag.Int("fps", 30, "video framerate used during encode")

	dataShards := flag.Int("data-shards", 0, "Reed-Solomon data shards (0=auto)")
	parityShards := flag.Int("parity-shards", 0, "Reed-Solomon parity shards (0=auto)")
	shardSize := flag.Int("shard-size", 0, "bytes per RS shard (0=auto)")
	audio := flag.Bool("audio", false, "extract data from audio track instead of video")

	showVersion := flag.Bool("version", false, "show version")

	flag.Parse()

	if *showVersion {
		fmt.Println("ystore-go decode " + version.Version)
		return
	}

	if *input == "" {
		log.Fatal("-input is required")
	}

	vc := vcodec.Config{
		CellSize:    *cellSize,
		BorderSize:  *borderSize,
		BitsPerCell: *bitsPerCell,
		Width:       *width,
		Height:      *height,
	}

	if err := vc.Validate(); err != nil {
		log.Fatalf("invalid video config: %v", err)
	}

	var rs *ecc.ReedSolomon
	if *dataShards > 0 && *parityShards > 0 && *shardSize > 0 {
		var err error
		rs, err = ecc.NewReedSolomon(*dataShards, *parityShards, *shardSize)
		if err != nil {
			log.Fatalf("creating RS decoder: %v", err)
		}
	} else {
		var err error
		rs, err = pipeline.MatchRSConfig(vc)
		if err != nil {
			log.Fatal(err)
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

	if err := pipeline.DecodeFile(cfg); err != nil {
		log.Fatalf("decode failed: %v", err)
	}

	log.Println("Done")
}
