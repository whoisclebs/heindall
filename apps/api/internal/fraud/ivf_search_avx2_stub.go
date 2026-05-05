//go:build !amd64

package fraud

import "unsafe"

var useIVFAVX2 = false

func quantizedBlock8DistancesAVX2(_ *int16, _ unsafe.Pointer, _ *int64) {}
