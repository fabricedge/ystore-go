package vcodec

import (
	"image"
)

func DecodeFrame(cfg Config, img image.Image) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cols := cfg.CellsPerRow()
	usedCells := cfg.UsedCells()
	cellValues := make([]uint8, usedCells)

	for idx := 0; idx < usedCells; idx++ {
		row := idx / cols
		col := idx % cols
		cellValues[idx] = cfg.ReadCell(img, col, row)
	}

	data := UnpackCellValuesToBytes(cellValues, cfg.BitsPerCell)
	return data, nil
}
