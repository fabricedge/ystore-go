package acodec

import (
	"fmt"
)

func Encode(data []byte, cfg MFSKConfig) ([]float64, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to encode")
	}

	samples, err := EncodeMFSK(cfg, data)
	if err != nil {
		return nil, fmt.Errorf("MFSK encode: %w", err)
	}

	return samples, nil
}

func EncodeToWAV(data []byte, cfg MFSKConfig) ([]byte, error) {
	samples, err := Encode(data, cfg)
	if err != nil {
		return nil, err
	}
	return SamplesToWAV(samples), nil
}
