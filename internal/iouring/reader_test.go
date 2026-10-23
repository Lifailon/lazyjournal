package iouring

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func openTestFile(t *testing.T, content string) (*os.File, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.log")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})
	return file, content
}

func TestFileReaderAtReadAt(t *testing.T) {
	file, content := openTestFile(t, "first\nsecond\nthird\n")
	reader := NewFileReaderAt(file)
	t.Cleanup(func() {
		_ = reader.Close()
	})

	buffer := make([]byte, len("second"))
	n, err := reader.ReadAt(buffer, int64(len(content[:len("first\n")])))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buffer) {
		t.Fatalf("ReadAt returned %d bytes, want %d", n, len(buffer))
	}
	if string(buffer) != "second" {
		t.Fatalf("ReadAt returned %q, want %q", buffer, "second")
	}
}

func TestFileReaderAtReadAtEOF(t *testing.T) {
	file, content := openTestFile(t, "first\nsecond\nthird\n")
	reader := NewFileReaderAt(file)
	t.Cleanup(func() {
		_ = reader.Close()
	})

	buffer := make([]byte, 8)
	offset := int64(len(content) - len("rd\n"))
	n, err := reader.ReadAt(buffer, offset)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt error = %v, want io.EOF", err)
	}
	if n != len("rd\n") {
		t.Fatalf("ReadAt returned %d bytes, want %d", n, len("rd\n"))
	}
	if string(buffer[:n]) != "rd\n" {
		t.Fatalf("ReadAt returned %q, want %q", buffer[:n], "rd\n")
	}
}

func TestFileReaderAtClose(t *testing.T) {
	file, _ := openTestFile(t, "data\n")
	reader := NewFileReaderAt(file)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}
