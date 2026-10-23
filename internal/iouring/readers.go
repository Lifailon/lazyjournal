package iouring

import (
	"errors"
	"io"
	"os"
)

var (
	_ io.Reader   = (*fastStdinReader)(nil)
	_ io.ReaderAt = (*fastFileReaderAt)(nil)
)

// fastStdinReader transparently falls back when io_uring is unavailable (for
// example under an older kernel or a restrictive container seccomp profile).
type fastStdinReader struct {
	ring *ioUringReader
}

func NewStdinReader() *fastStdinReader {
	ring, _ := newIOUringReader(int(os.Stdin.Fd()))
	return &fastStdinReader{ring: ring}
}

func (reader *fastStdinReader) Read(buffer []byte) (int, error) {
	if reader.ring != nil {
		n, err := reader.ring.Read(buffer)
		if err == nil || errors.Is(err, io.EOF) {
			return n, err
		}
		_ = reader.ring.Close()
		reader.ring = nil
	}
	return os.Stdin.Read(buffer)
}

func (reader *fastStdinReader) Close() error {
	if reader.ring != nil {
		return reader.ring.Close()
	}
	return nil
}

type fastFileReaderAt struct {
	file *os.File
	ring *ioUringReader
}

func NewFileReaderAt(file *os.File) *fastFileReaderAt {
	ring, _ := newIOUringReader(int(file.Fd()))
	return &fastFileReaderAt{file: file, ring: ring}
}

func (reader *fastFileReaderAt) ReadAt(buffer []byte, offset int64) (int, error) {
	if reader.ring != nil {
		n, err := reader.ring.ReadAt(buffer, offset)
		if err == nil || errors.Is(err, io.EOF) && n > 0 {
			return n, err
		}
		_ = reader.ring.Close()
		reader.ring = nil
	}
	return reader.file.ReadAt(buffer, offset)
}

func (reader *fastFileReaderAt) Close() error {
	if reader.ring != nil {
		return reader.ring.Close()
	}
	return nil
}
