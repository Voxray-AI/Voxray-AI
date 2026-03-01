// Package audio provides WAV decoding utilities.
package audio

import (
	"encoding/binary"
	"errors"
)

// DecodeWAVToPCM extracts raw 16-bit little-endian PCM and sample rate from WAV bytes.
// Returns (pcm, sampleRate, nil) or (nil, 0, error) for invalid/unsupported WAV.
// Supports standard PCM WAV with "fmt " and "data" chunks.
func DecodeWAVToPCM(wav []byte) (pcm []byte, sampleRate int, err error) {
	const (
		riff    = "RIFF"
		wave    = "WAVE"
		chunkFmt  = "fmt "
		data    = "data"
	)
	if len(wav) < 44 {
		return nil, 0, errors.New("wav: too short")
	}
	if string(wav[0:4]) != riff || string(wav[8:12]) != wave {
		return nil, 0, errors.New("wav: invalid RIFF/WAVE header")
	}
	pos := 12
	for pos+8 <= len(wav) {
		chunkID := string(wav[pos : pos+4])
		chunkSize := binary.LittleEndian.Uint32(wav[pos+4 : pos+8])
		pos += 8
		// Be tolerant of incorrect or oversized chunk sizes by clamping
		if pos+int(chunkSize) > len(wav) {
			chunkSize = uint32(len(wav) - pos)
		}
		if chunkSize == 0 {
			continue
		}
		payload := wav[pos : pos+int(chunkSize)]
		switch chunkID {
		case chunkFmt:
			if chunkSize >= 16 {
				// audio format (2), num channels (2), sample rate (4)
				format := binary.LittleEndian.Uint16(payload[0:2])
				if format != 1 {
					return nil, 0, errors.New("wav: only PCM (format 1) supported")
				}
				sampleRate = int(binary.LittleEndian.Uint32(payload[4:8]))
			}
		case data:
			pcm = make([]byte, len(payload))
			copy(pcm, payload)
		}
		pos += int(chunkSize)
	}
	if pcm == nil {
		return nil, 0, errors.New("wav: no data chunk")
	}
	if sampleRate == 0 {
		sampleRate = 48000
	}
	return pcm, sampleRate, nil
}

