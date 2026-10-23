//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

func platformIOUringMmap(fd, offset, size int) ([]byte, error) {
	return syscall.Mmap(
		fd,
		int64(offset),
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
}

func platformIOUringSetup(syscallNumber, entries uintptr, params unsafe.Pointer) (uintptr, error) {
	fd, _, errno := syscall.Syscall6(
		syscallNumber,
		entries,
		uintptr(params),
		0, 0, 0, 0,
	)
	if errno != 0 {
		return 0, errno
	}
	return fd, nil
}

func platformIOUringEnter(syscallNumber uintptr, fd, toSubmit, minComplete int, flags uintptr) error {
	_, _, errno := syscall.Syscall6(
		syscallNumber,
		uintptr(fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		flags,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func platformIOUringMunmap(memory []byte) error {
	return syscall.Munmap(memory)
}

func platformIOUringClose(fd int) error {
	return syscall.Close(fd)
}
