package recording

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// writeEmptyWAVHeader writes a placeholder RIFF/WAVE header for 16-bit PCM mono.
// finalizeWAV must be called later to fix sizes.
func writeEmptyWAVHeader(w io.Writer, sampleRate int) error {
	var (
		audioFormat   uint16 = 1  // PCM
		numChannels   uint16 = 1  // mono
		bitsPerSample uint16 = 16 // 16-bit
		byteRate             = uint32(sampleRate) * uint32(numChannels) * uint32(bitsPerSample/8)
		blockAlign           = numChannels * (bitsPerSample / 8)
	)

	if _, err := w.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("WAVE")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, audioFormat); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, numChannels); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, bitsPerSample); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	return nil
}

// finalizeWAV fixes the RIFF and data chunk sizes for a 16-bit PCM mono WAV file.
func finalizeWAV(f *os.File, numSamples int64, sampleRate int, numChannels int) error {
	if numChannels <= 0 {
		return fmt.Errorf("invalid channels: %d", numChannels)
	}
	if numSamples < 0 {
		return fmt.Errorf("invalid samples: %d", numSamples)
	}
	bitsPerSample := 16
	dataSize := uint32(numSamples * int64(numChannels) * int64(bitsPerSample/8))
	riffSize := 4 + 8 + 16 + 8 + dataSize

	if _, err := f.Seek(4, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, riffSize); err != nil {
		return err
	}
	if _, err := f.Seek(40, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, dataSize); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	_ = sampleRate
	return nil
}

