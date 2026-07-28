package formats

import (
	"encoding/binary"
	"fmt"
)

const FrameHeaderSize = 8

type FrameHeader struct {
	Magic       uint32
	FrameNumber uint32
	TotalFrames uint32
	DataLength  uint32
}

var FrameMagic = uint32(0x5954594D)

func NewFrameHeader(frameNum, totalFrames, dataLen uint32) FrameHeader {
	return FrameHeader{
		Magic:       FrameMagic,
		FrameNumber: frameNum,
		TotalFrames: totalFrames,
		DataLength:  dataLen,
	}
}

func (h FrameHeader) MarshalBinary() []byte {
	buf := make([]byte, FrameHeaderSize)
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], h.FrameNumber)
	return buf[:8]
}

func (h FrameHeader) MarshalFull(frames uint32, dataLen uint32) []byte {
	buf := make([]byte, FrameHeaderSize*2)
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], h.FrameNumber)
	binary.LittleEndian.PutUint32(buf[8:12], frames)
	binary.LittleEndian.PutUint32(buf[12:16], dataLen)
	return buf
}

func UnmarshalFrameHeader(data []byte) (FrameHeader, bool) {
	if len(data) < FrameHeaderSize {
		return FrameHeader{}, false
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != FrameMagic {
		return FrameHeader{}, false
	}
	return FrameHeader{
		Magic:       magic,
		FrameNumber: binary.LittleEndian.Uint32(data[4:8]),
	}, true
}

func UnmarshalFrameFull(data []byte) (FrameHeader, uint32, uint32, error) {
	if len(data) < FrameHeaderSize*2 {
		return FrameHeader{}, 0, 0, fmt.Errorf("frame header too short: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != FrameMagic {
		return FrameHeader{}, 0, 0, fmt.Errorf("invalid frame magic: 0x%08X", magic)
	}
	return FrameHeader{
		Magic:       magic,
		FrameNumber: binary.LittleEndian.Uint32(data[4:8]),
	}, binary.LittleEndian.Uint32(data[8:12]), binary.LittleEndian.Uint32(data[12:16]), nil
}
