package fraud

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

var useIVFAVX2 = cpu.X86.HasAVX2

//go:noescape
func quantizedBlock8DistancesAVX2(query *int16, block unsafe.Pointer, out *int64)

//go:noescape
func quantizedBlock32DistancesAVX2(query *int16, block unsafe.Pointer, out *int64)

//go:noescape
func quantized8DistancesRowMajorAVX2(query *int16, vectors unsafe.Pointer, out *int64)
