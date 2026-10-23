//go:build !linux

package main

import (
	"errors"
	"unsafe"
)

var errIOUringUnsupported = errors.New("io_uring is only available on Linux")

func platformIOUringMmap(_, _, _ int) ([]byte, error) {
	return nil, errIOUringUnsupported
}

func platformIOUringSetup(_, _ uintptr, _ unsafe.Pointer) (uintptr, error) {
	return 0, errIOUringUnsupported
}

func platformIOUringEnter(_ uintptr, _, _, _ int, _ uintptr) error {
	return errIOUringUnsupported
}

func platformIOUringMunmap(_ []byte) error {
	return nil
}

func platformIOUringClose(_ int) error {
	return nil
}
