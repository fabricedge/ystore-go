package vcodec

import (
	"fmt"
	"image"
	"image/color"
)

const (
	DefaultCellSize    = 24
	DefaultBorderSize  = 24
	DefaultBitsPerCell = 2
	DefaultGuardPixels = 4
	VideoWidth         = 1920
	VideoHeight        = 1080
)

type Config struct {
	CellSize    int
	BorderSize  int
	BitsPerCell int
}

func DefaultConfig() Config {
	return Config{
		CellSize:    DefaultCellSize,
		BorderSize:  DefaultBorderSize,
		BitsPerCell: DefaultBitsPerCell,
	}
}

func (c Config) CellsPerRow() int {
	usable := VideoWidth - 2*c.BorderSize
	return usable / c.CellSize
}

func (c Config) CellsPerCol() int {
	usable := VideoHeight - 2*c.BorderSize
	return usable / c.CellSize
}

func (c Config) TotalCells() int {
	return c.CellsPerRow() * c.CellsPerCol()
}

func (c Config) MaxBitsPerFrame() int {
	return c.TotalCells() * c.BitsPerCell
}

func (c Config) MaxBytesPerFrame() int {
	return c.MaxBitsPerFrame() / 8
}

func (c Config) UsedCells() int {
	return c.MaxBytesPerFrame() * 8 / c.BitsPerCell
}

func (c Config) Validate() error {
	if c.CellSize < 8 || c.CellSize > 200 {
		return fmt.Errorf("CellSize must be 8-200")
	}
	if c.BorderSize < 0 || c.BorderSize > 200 {
		return fmt.Errorf("BorderSize must be 0-200")
	}
	if c.BitsPerCell < 1 || c.BitsPerCell > 8 {
		return fmt.Errorf("BitsPerCell must be 1-8")
	}
	if c.CellsPerRow() < 1 || c.CellsPerCol() < 1 {
		return fmt.Errorf("grid too small: %dx%d cells", c.CellsPerRow(), c.CellsPerCol())
	}
	return nil
}

func (c Config) CellRect(col, row int) image.Rectangle {
	x0 := c.BorderSize + col*c.CellSize
	y0 := c.BorderSize + row*c.CellSize
	return image.Rect(x0, y0, x0+c.CellSize, y0+c.CellSize)
}

func guardPixels(cellSize int) int {
	g := cellSize / 6
	if g < 2 {
		g = 2
	}
	if g > 8 {
		g = 8
	}
	return g
}

func (c Config) NewFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, VideoWidth, VideoHeight))
	bg := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	for y := 0; y < VideoHeight; y++ {
		for x := 0; x < VideoWidth; x++ {
			img.Set(x, y, bg)
		}
	}
	return img
}

func (c Config) valueToRGBA(value uint8) color.RGBA {
	maxVal := uint8((1 << c.BitsPerCell) - 1)
	if value > maxVal {
		value = maxVal
	}
	y := uint8(32 + (int(value)*192)/int(maxVal))
	return color.RGBA{R: y, G: y, B: y, A: 255}
}

func (c Config) lumToValue(lum uint32) uint8 {
	maxVal := (1 << c.BitsPerCell) - 1
	if lum <= 32 {
		return 0
	}
	if lum >= 224 {
		return uint8(maxVal)
	}
	step := 192 / (maxVal + 1)
	idx := (lum - 32) / uint32(step)
	if idx > uint32(maxVal) {
		idx = uint32(maxVal)
	}
	return uint8(idx)
}

func (c Config) DrawCell(img *image.RGBA, col, row int, value uint8) {
	rgba := c.valueToRGBA(value)
	rect := c.CellRect(col, row)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, y, rgba)
		}
	}
}

func (c Config) ReadCell(img image.Image, col, row int) uint8 {
	rect := c.CellRect(col, row)
	guard := guardPixels(c.CellSize)

	x0 := rect.Min.X + guard
	y0 := rect.Min.Y + guard
	x1 := rect.Max.X - guard
	y1 := rect.Max.Y - guard

	if x0 >= x1 || y0 >= y1 {
		x0, y0 = rect.Min.X, rect.Min.Y
		x1, y1 = rect.Max.X, rect.Max.Y
	}

	var sum uint64
	count := uint64(0)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := (299*r + 587*g + 114*b) / 1000
			sum += uint64(lum >> 8)
			count++
		}
	}

	if count == 0 {
		return 0
	}
	avg := uint32(sum / count)
	return c.lumToValue(avg)
}

func PackBytesToCellValues(data []byte, bitsPerCell int) ([]uint8, error) {
	if bitsPerCell < 1 || bitsPerCell > 8 {
		return nil, fmt.Errorf("bitsPerCell must be 1-8")
	}

	totalBits := len(data) * 8
	if totalBits%bitsPerCell != 0 {
		return nil, fmt.Errorf("data length %d bytes (%d bits) not divisible by %d bits/cell",
			len(data), totalBits, bitsPerCell)
	}

	numCells := totalBits / bitsPerCell
	values := make([]uint8, numCells)

	for i := 0; i < numCells; i++ {
		var val uint8
		for j := 0; j < bitsPerCell; j++ {
			bitPos := i*bitsPerCell + j
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			val <<= 1
			if byteIdx < len(data) {
				val |= (data[byteIdx] >> bitIdx) & 1
			}
		}
		values[i] = val
	}
	return values, nil
}

func UnpackCellValuesToBytes(values []uint8, bitsPerCell int) []byte {
	if len(values) == 0 || bitsPerCell < 1 || bitsPerCell > 8 {
		return nil
	}

	totalBits := len(values) * bitsPerCell
	numBytes := (totalBits + 7) / 8
	data := make([]byte, numBytes)

	for i, val := range values {
		for j := 0; j < bitsPerCell; j++ {
			bitPos := i*bitsPerCell + j
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			bit := (val >> (bitsPerCell - 1 - j)) & 1
			data[byteIdx] |= bit << bitIdx
		}
	}
	return data
}
