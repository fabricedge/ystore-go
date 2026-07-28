package acodec

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	SampleRate      = 44100
	BitsPerSample   = 16
	NumChannels     = 1
	SymbolSamples   = 4096
	PreambleSymbols = 4
)

type MFSKConfig struct {
	FreqStart  float64
	FreqEnd    float64
	NumTones   int
	BitsPerSym int
}

func DefaultMFSKConfig() MFSKConfig {
	return MFSKConfig{
		FreqStart:  1000,
		FreqEnd:    4000,
		NumTones:   64,
		BitsPerSym: 6,
	}
}

func (c MFSKConfig) TonesPerSec() float64 {
	return float64(SampleRate) / float64(SymbolSamples)
}

func (c MFSKConfig) DataRate() float64 {
	return c.TonesPerSec() * float64(c.BitsPerSym)
}

func (c MFSKConfig) ToneFrequencies() []float64 {
	freqs := make([]float64, c.NumTones)
	for i := 0; i < c.NumTones; i++ {
		freqs[i] = c.FreqStart + (c.FreqEnd-c.FreqStart)*float64(i)/float64(c.NumTones-1)
	}
	return freqs
}

func GeneratePreamble(cfg MFSKConfig) []float64 {
	samples := make([]float64, 0, PreambleSymbols*SymbolSamples)
	freqs := cfg.ToneFrequencies()

	for s := 0; s < PreambleSymbols; s++ {
		toneFreq := freqs[s%len(freqs)]
		sym := GenerateTone(toneFreq, SymbolSamples)
		samples = append(samples, sym...)
	}
	return samples
}

func GenerateTone(freq float64, numSamples int) []float64 {
	samples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / SampleRate
		window := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(numSamples)))
		samples[i] = math.Sin(2*math.Pi*freq*t) * window
	}
	return samples
}

func prependLength(data []byte) []byte {
	out := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(data)))
	copy(out[4:], data)
	return out
}

func readLength(data []byte) (uint32, []byte) {
	if len(data) < 4 {
		return 0, data
	}
	length := binary.LittleEndian.Uint32(data[0:4])
	if int(length) > len(data)-4 {
		length = uint32(len(data) - 4)
	}
	return length, data[4 : 4+length]
}

func EncodeMFSK(cfg MFSKConfig, data []byte) ([]float64, error) {
	freqs := cfg.ToneFrequencies()

	dataWithLen := prependLength(data)
	totalBits := len(dataWithLen) * 8
	paddedBits := ((totalBits + cfg.BitsPerSym - 1) / cfg.BitsPerSym) * cfg.BitsPerSym
	paddedBytes := (paddedBits + 7) / 8

	padded := make([]byte, paddedBytes)
	copy(padded, dataWithLen)

	numSymbols := paddedBits / cfg.BitsPerSym

	preamble := GeneratePreamble(cfg)
	output := make([]float64, 0, len(preamble)+numSymbols*SymbolSamples)
	output = append(output, preamble...)

	for sym := 0; sym < numSymbols; sym++ {
		var symVal int
		for j := 0; j < cfg.BitsPerSym; j++ {
			bitPos := sym*cfg.BitsPerSym + j
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			if byteIdx < len(padded) {
				symVal <<= 1
				symVal |= int((padded[byteIdx] >> bitIdx) & 1)
			}
		}

		if symVal >= len(freqs) {
			symVal = len(freqs) - 1
		}

		tone := GenerateTone(freqs[symVal], SymbolSamples)
		output = append(output, tone...)
	}

	return output, nil
}

func goertzel(samples []float64, targetFreq float64) float64 {
	N := len(samples)
	k := int(0.5 + float64(N)*targetFreq/SampleRate)
	if k >= N {
		k = N - 1
	}
	omega := 2 * math.Pi * float64(k) / float64(N)
	coeff := 2 * math.Cos(omega)

	var s0, s1, s2 float64
	for _, sample := range samples {
		s0 = sample + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	return s1*s1 + s2*s2 - coeff*s1*s2
}

func DecodeMFSK(cfg MFSKConfig, samples []float64) ([]byte, error) {
	freqs := cfg.ToneFrequencies()

	if len(samples) < PreambleSymbols*SymbolSamples+SymbolSamples {
		return nil, fmt.Errorf("audio too short: %d samples", len(samples))
	}

	dataSamples := samples[PreambleSymbols*SymbolSamples:]
	numSymbols := len(dataSamples) / SymbolSamples
	totalBits := numSymbols * cfg.BitsPerSym
	numBytes := (totalBits + 7) / 8

	data := make([]byte, numBytes)

	for sym := 0; sym < numSymbols; sym++ {
		start := sym * SymbolSamples
		end := start + SymbolSamples
		if end > len(dataSamples) {
			break
		}
		symSamples := dataSamples[start:end]

		bestFreq := 0
		bestPower := -1.0
		for fi := 0; fi < len(freqs); fi++ {
			power := goertzel(symSamples, freqs[fi])
			if power > bestPower {
				bestPower = power
				bestFreq = fi
			}
		}

		for j := 0; j < cfg.BitsPerSym; j++ {
			bitPos := sym*cfg.BitsPerSym + j
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			bit := (bestFreq >> (cfg.BitsPerSym - 1 - j)) & 1
			data[byteIdx] |= byte(bit) << bitIdx
		}
	}

	actualLen, payload := readLength(data)
	data = make([]byte, actualLen)
	copy(data, payload)

	return data, nil
}

func SamplesToWAV(samples []float64) []byte {
	numSamples := len(samples)
	dataSize := numSamples * BitsPerSample / 8
	headerSize := 44
	totalSize := headerSize + dataSize

	buf := make([]byte, totalSize)

	copy(buf[0:4], []byte("RIFF"))
	writeLE32(buf[4:8], uint32(totalSize-8))
	copy(buf[8:12], []byte("WAVE"))
	copy(buf[12:16], []byte("fmt "))
	writeLE32(buf[16:20], 16)
	writeLE16(buf[20:22], 1)
	writeLE16(buf[22:24], uint16(NumChannels))
	writeLE32(buf[24:28], uint32(SampleRate))
	blockAlign := uint16(NumChannels * BitsPerSample / 8)
	byteRate := uint32(SampleRate) * uint32(blockAlign)
	writeLE32(buf[28:32], byteRate)
	writeLE16(buf[32:34], blockAlign)
	writeLE16(buf[34:36], uint16(BitsPerSample))
	copy(buf[36:40], []byte("data"))
	writeLE32(buf[40:44], uint32(dataSize))

	for i := 0; i < numSamples; i++ {
		sample := int16(samples[i] * 32767)
		if samples[i] > 1 {
			sample = 32767
		} else if samples[i] < -1 {
			sample = -32768
		}
		offset := 44 + i*2
		writeLE16(buf[offset:offset+2], uint16(sample))
	}

	return buf
}

func WAVToSamples(wavData []byte) ([]float64, error) {
	if len(wavData) < 44 {
		return nil, fmt.Errorf("WAV too short: %d bytes", len(wavData))
	}

	if string(wavData[0:4]) != "RIFF" || string(wavData[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid WAV header")
	}

	numChannels := int(readLE16(wavData[22:24]))
	sampleRate := int(readLE32(wavData[24:28]))
	bitsPerSample := int(readLE16(wavData[34:36]))

	if sampleRate != SampleRate {
		return nil, fmt.Errorf("unexpected sample rate: %d (want %d)", sampleRate, SampleRate)
	}
	if bitsPerSample != BitsPerSample {
		return nil, fmt.Errorf("unexpected bits per sample: %d (want %d)", bitsPerSample, BitsPerSample)
	}

	dataSize := int(readLE32(wavData[40:44]))
	if len(wavData) < 44+dataSize {
		dataSize = len(wavData) - 44
	}

	numSamples := dataSize / (bitsPerSample / 8) / numChannels
	samples := make([]float64, numSamples)

	for i := 0; i < numSamples; i++ {
		var sum int
		for ch := 0; ch < numChannels; ch++ {
			chOffset := 44 + (i*numChannels+ch)*2
			if chOffset+1 >= len(wavData) {
				break
			}
			sum += int(int16(readLE16(wavData[chOffset : chOffset+2])))
		}
		samples[i] = float64(sum/numChannels) / 32767.0
	}

	return samples, nil
}

func writeLE16(buf []byte, v uint16) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
}

func writeLE32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

func readLE16(buf []byte) uint16 {
	return uint16(buf[0]) | uint16(buf[1])<<8
}

func readLE32(buf []byte) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}
