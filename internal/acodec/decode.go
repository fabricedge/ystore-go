package acodec

import "fmt"

func DecodeFromWAV(wavData []byte, cfg MFSKConfig) ([]byte, error) {
	samples, err := WAVToSamples(wavData)
	if err != nil {
		return nil, fmt.Errorf("WAV parse: %w", err)
	}

	data, err := DecodeMFSK(cfg, samples)
	if err != nil {
		return nil, fmt.Errorf("MFSK decode: %w", err)
	}

	return data, nil
}

func DecodeFromSamples(samples []float64, cfg MFSKConfig) ([]byte, error) {
	return DecodeMFSK(cfg, samples)
}
