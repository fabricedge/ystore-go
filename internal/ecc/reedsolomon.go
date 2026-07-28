package ecc

import (
	"fmt"

	rs "github.com/klauspost/reedsolomon"
)

type ReedSolomon struct {
	enc          rs.Encoder
	DataShards   int
	ParityShards int
	ShardSize    int
}

func NewReedSolomon(dataShards, parityShards, shardSize int) (*ReedSolomon, error) {
	if dataShards <= 0 || parityShards <= 0 || shardSize <= 0 {
		return nil, fmt.Errorf("invalid RS parameters: data=%d parity=%d shardSize=%d",
			dataShards, parityShards, shardSize)
	}
	enc, err := rs.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("reedsolomon.New: %w", err)
	}
	return &ReedSolomon{enc: enc, DataShards: dataShards, ParityShards: parityShards, ShardSize: shardSize}, nil
}

func (r *ReedSolomon) DataBytes() int {
	return r.DataShards * r.ShardSize
}

func (r *ReedSolomon) TotalBytes() int {
	return (r.DataShards + r.ParityShards) * r.ShardSize
}

func (r *ReedSolomon) Encode(data []byte) ([]byte, error) {
	expected := r.DataBytes()
	if len(data) != expected {
		return nil, fmt.Errorf("RS encode: expected %d bytes, got %d", expected, len(data))
	}

	totalShards := r.DataShards + r.ParityShards
	shards := make([][]byte, totalShards)

	for i := 0; i < totalShards; i++ {
		shards[i] = make([]byte, r.ShardSize)
	}

	for i := 0; i < r.DataShards; i++ {
		start := i * r.ShardSize
		copy(shards[i], data[start:start+r.ShardSize])
	}

	if err := r.enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("RS Encode: %w", err)
	}

	out := make([]byte, r.TotalBytes())
	idx := 0
	for i := 0; i < totalShards; i++ {
		copy(out[idx:], shards[i])
		idx += r.ShardSize
	}
	return out, nil
}

func (r *ReedSolomon) Decode(encoded []byte) ([]byte, error) {
	expected := r.TotalBytes()
	if len(encoded) != expected {
		return nil, fmt.Errorf("RS decode: expected %d bytes, got %d", expected, len(encoded))
	}

	totalShards := r.DataShards + r.ParityShards
	shards := make([][]byte, totalShards)

	for i := 0; i < totalShards; i++ {
		start := i * r.ShardSize
		shards[i] = encoded[start : start+r.ShardSize]
	}

	if ok, _ := r.enc.Verify(shards); !ok {
		if err := r.enc.Reconstruct(shards); err != nil {
			return nil, fmt.Errorf("RS Reconstruct: %w", err)
		}
	}

	out := make([]byte, r.DataBytes())
	idx := 0
	for i := 0; i < r.DataShards; i++ {
		copy(out[idx:], shards[i])
		idx += r.ShardSize
	}
	return out, nil
}
