package vcodec

import (
	"fmt"
	"image"
)

func EncodeFrame(cfg Config, encodedData []byte) (*image.RGBA, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	expectedBytes := cfg.MaxBytesPerFrame()
	if len(encodedData) != expectedBytes {
		return nil, fmt.Errorf("EncodeFrame: expected %d bytes, got %d",
			expectedBytes, len(encodedData))
	}

	usedCells := cfg.UsedCells()
	cellValues, err := PackBytesToCellValues(encodedData[:expectedBytes], cfg.BitsPerCell)
	if err != nil {
		return nil, err
	}

	if len(cellValues) != usedCells {
		return nil, fmt.Errorf("EncodeFrame: got %d cell values, expected %d", len(cellValues), usedCells)
	}

	img := cfg.NewFrame()
	cols := cfg.CellsPerRow()

	for idx := 0; idx < usedCells; idx++ {
		row := idx / cols
		col := idx % cols
		cfg.DrawCell(img, col, row, cellValues[idx])
	}

	return img, nil
}
