package iouring

import (
	"io"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	ioUringSetupSyscall = 425
	ioUringEnterSyscall = 426
	ioUringOffSqRing    = 0
	ioUringOffCqRing    = 0x08000000
	ioUringOffSqes      = 0x10000000
	ioUringFeatSingleMM = 1
	ioUringEnterGetEvts = 1
	ioUringOpRead       = 22
)

type ioUringSqOffsets struct {
	head, tail, ringMask, ringEntries, flags, dropped, array, reserved uint32
	userAddr                                                           uint64
}

type ioUringCqOffsets struct {
	head, tail, ringMask, ringEntries, overflow, cqes, flags, reserved uint32
	userAddr                                                           uint64
}

type ioUringParams struct {
	sqEntries, cqEntries, flags, sqThreadCPU, sqThreadIdle, features, wqFd uint32
	reserved                                                               [3]uint32
	sqOffset                                                               ioUringSqOffsets
	cqOffset                                                               ioUringCqOffsets
}

type ioUringSqe struct {
	opcode, flags         uint8
	ioprio                uint16
	fd                    int32
	offset, address       uint64
	length, rwFlags       uint32
	userData              uint64
	bufIndex, personality uint16
	spliceFdIn            int32
	address3, pad         uint64
}

type ioUringCqe struct {
	userData uint64
	result   int32
	flags    uint32
}

type ioUringReader struct {
	fd         int
	inputFd    int32
	sqRing     []byte
	cqRing     []byte
	sqesMemory []byte
	singleMmap bool
	sqHead     *uint32
	sqTail     *uint32
	sqMask     *uint32
	sqEntries  *uint32
	sqArray    []uint32
	sqes       []ioUringSqe
	cqHead     *uint32
	cqTail     *uint32
	cqMask     *uint32
	cqes       []ioUringCqe
}

func ioUringSyscalls() (uintptr, uintptr, bool) {
	if runtime.GOOS != "linux" {
		return 0, 0, false
	}
	switch runtime.GOARCH {
	case "mips", "mipsle":
		return ioUringSetupSyscall + 4000, ioUringEnterSyscall + 4000, true
	case "mips64", "mips64le":
		return ioUringSetupSyscall + 5000, ioUringEnterSyscall + 5000, true
	default:
		return ioUringSetupSyscall, ioUringEnterSyscall, true
	}
}

func mmapUring(fd int, offset, size int) ([]byte, error) {
	if size <= 0 {
		return nil, syscall.EINVAL
	}
	return platformIOUringMmap(fd, offset, size)
}

func uint32At(memory []byte, offset uint32) *uint32 {
	return (*uint32)(unsafe.Pointer(&memory[int(offset)]))
}

func newIOUringReader(inputFd int) (_ *ioUringReader, err error) {
	setupSyscall, _, supported := ioUringSyscalls()
	if !supported {
		return nil, syscall.ENOSYS
	}

	params := ioUringParams{}
	fd, setupErr := platformIOUringSetup(
		setupSyscall,
		8,
		unsafe.Pointer(&params),
	)
	if setupErr != nil {
		return nil, setupErr
	}

	ring := &ioUringReader{fd: int(fd), inputFd: int32(inputFd)}
	defer func() {
		if err != nil {
			_ = ring.Close()
		}
	}()

	sqRingSize := int(params.sqOffset.array) + int(params.sqEntries)*4
	cqRingSize := int(params.cqOffset.cqes) + int(params.cqEntries)*int(unsafe.Sizeof(ioUringCqe{}))
	if params.features&ioUringFeatSingleMM != 0 {
		ring.singleMmap = true
		ring.sqRing, err = mmapUring(ring.fd, ioUringOffSqRing, max(sqRingSize, cqRingSize))
		if err != nil {
			return nil, err
		}
		ring.cqRing = ring.sqRing
	} else {
		ring.sqRing, err = mmapUring(ring.fd, ioUringOffSqRing, sqRingSize)
		if err != nil {
			return nil, err
		}
		ring.cqRing, err = mmapUring(ring.fd, ioUringOffCqRing, cqRingSize)
		if err != nil {
			return nil, err
		}
	}

	sqesSize := int(params.sqEntries) * int(unsafe.Sizeof(ioUringSqe{}))
	ring.sqesMemory, err = mmapUring(ring.fd, ioUringOffSqes, sqesSize)
	if err != nil {
		return nil, err
	}

	ring.sqHead = uint32At(ring.sqRing, params.sqOffset.head)
	ring.sqTail = uint32At(ring.sqRing, params.sqOffset.tail)
	ring.sqMask = uint32At(ring.sqRing, params.sqOffset.ringMask)
	ring.sqEntries = uint32At(ring.sqRing, params.sqOffset.ringEntries)
	ring.cqHead = uint32At(ring.cqRing, params.cqOffset.head)
	ring.cqTail = uint32At(ring.cqRing, params.cqOffset.tail)
	ring.cqMask = uint32At(ring.cqRing, params.cqOffset.ringMask)
	ring.sqArray = unsafe.Slice(
		(*uint32)(unsafe.Pointer(&ring.sqRing[int(params.sqOffset.array)])),
		int(params.sqEntries),
	)
	ring.sqes = unsafe.Slice(
		(*ioUringSqe)(unsafe.Pointer(&ring.sqesMemory[0])),
		int(params.sqEntries),
	)
	ring.cqes = unsafe.Slice(
		(*ioUringCqe)(unsafe.Pointer(&ring.cqRing[int(params.cqOffset.cqes)])),
		int(params.cqEntries),
	)
	return ring, nil
}

func (ring *ioUringReader) readAtOffset(buffer []byte, offset uint64) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	_, enterSyscall, supported := ioUringSyscalls()
	if !supported || ring.fd < 0 {
		return 0, syscall.ENOSYS
	}

	tail := atomic.LoadUint32(ring.sqTail)
	head := atomic.LoadUint32(ring.sqHead)
	if tail-head >= atomic.LoadUint32(ring.sqEntries) {
		return 0, syscall.EBUSY
	}
	index := tail & atomic.LoadUint32(ring.sqMask)
	sqe := &ring.sqes[index]
	*sqe = ioUringSqe{
		opcode:   ioUringOpRead,
		fd:       ring.inputFd,
		offset:   offset,
		address:  uint64(uintptr(unsafe.Pointer(&buffer[0]))),
		length:   uint32(len(buffer)),
		userData: uint64(tail) + 1,
	}
	ring.sqArray[index] = index
	atomic.StoreUint32(ring.sqTail, tail+1)

	enterErr := platformIOUringEnter(
		enterSyscall,
		ring.fd,
		1,
		1,
		ioUringEnterGetEvts,
	)
	runtime.KeepAlive(buffer)
	if enterErr != nil {
		return 0, enterErr
	}

	cqHead := atomic.LoadUint32(ring.cqHead)
	cqTail := atomic.LoadUint32(ring.cqTail)
	if cqHead == cqTail {
		return 0, syscall.EIO
	}
	cqe := ring.cqes[cqHead&atomic.LoadUint32(ring.cqMask)]
	atomic.StoreUint32(ring.cqHead, cqHead+1)
	if cqe.result < 0 {
		return 0, syscall.Errno(-cqe.result)
	}
	if cqe.result == 0 {
		return 0, io.EOF
	}
	return int(cqe.result), nil
}

func (ring *ioUringReader) Read(buffer []byte) (int, error) {
	return ring.readAtOffset(buffer, ^uint64(0))
}

func (ring *ioUringReader) ReadAt(buffer []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, syscall.EINVAL
	}
	total := 0
	for total < len(buffer) {
		n, err := ring.readAtOffset(buffer[total:], uint64(offset)+uint64(total))
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.EOF
		}
	}
	return total, nil
}

func (ring *ioUringReader) Close() error {
	if len(ring.sqesMemory) != 0 {
		_ = platformIOUringMunmap(ring.sqesMemory)
		ring.sqesMemory = nil
	}
	if len(ring.cqRing) != 0 && !ring.singleMmap {
		_ = platformIOUringMunmap(ring.cqRing)
		ring.cqRing = nil
	}
	if len(ring.sqRing) != 0 {
		_ = platformIOUringMunmap(ring.sqRing)
		ring.sqRing = nil
	}
	if ring.fd >= 0 {
		err := platformIOUringClose(ring.fd)
		ring.fd = -1
		return err
	}
	return nil
}
