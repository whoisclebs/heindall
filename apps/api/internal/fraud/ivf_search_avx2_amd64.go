package fraud

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

var useIVFAVX2 = cpu.X86.HasAVX2

//go:noescape
func quantizedBlock8DistancesAVX2(query *int16, block unsafe.Pointer, out *int64)
