#include "textflag.h"

#define STEP(qoff, boff) \
	VPBROADCASTW qoff(AX), X2; \
	VPMOVSXWD X2, Y2; \
	VMOVDQU boff(BX), X3; \
	VPMOVSXWD X3, Y3; \
	VPSUBD Y3, Y2, Y4; \
	VPMULLD Y4, Y4, Y4; \
	VPMOVSXDQ X4, Y5; \
	VEXTRACTI128 $1, Y4, X6; \
	VPMOVSXDQ X6, Y6; \
	VPADDQ Y5, Y0, Y0; \
	VPADDQ Y6, Y1, Y1

// func quantizedBlock8DistancesAVX2(query *int16, block unsafe.Pointer, out *int64)
TEXT ·quantizedBlock8DistancesAVX2(SB), NOSPLIT, $0-24
	MOVQ query+0(FP), AX
	MOVQ block+8(FP), BX
	MOVQ out+16(FP), CX

	VPXOR Y0, Y0, Y0 // accum lanes 0..3 as int64
	VPXOR Y1, Y1, Y1 // accum lanes 4..7 as int64

	// Same dimension order as quantizedBlockLaneDistanceUnsafe.
	STEP(10, 80)  // dim 5
	STEP(12, 96)  // dim 6
	STEP(4, 32)   // dim 2
	STEP(0, 0)    // dim 0
	STEP(14, 112) // dim 7
	STEP(16, 128) // dim 8
	STEP(24, 192) // dim 12
	STEP(2, 16)   // dim 1
	STEP(6, 48)   // dim 3
	STEP(8, 64)   // dim 4
	STEP(18, 144) // dim 9
	STEP(20, 160) // dim 10
	STEP(22, 176) // dim 11
	STEP(26, 208) // dim 13

	VMOVDQU Y0, 0(CX)
	VMOVDQU Y1, 32(CX)
	VZEROUPPER
	RET

#undef STEP

#define ROWSTEP(qoff, boff) \
	VPBROADCASTW qoff(AX), X2; \
	VPMOVSXWD X2, Y2; \
	VPINSRW $0, boff(BX), X3, X3; \
	VPINSRW $1, boff+28(BX), X3, X3; \
	VPINSRW $2, boff+56(BX), X3, X3; \
	VPINSRW $3, boff+84(BX), X3, X3; \
	VPINSRW $4, boff+112(BX), X3, X3; \
	VPINSRW $5, boff+140(BX), X3, X3; \
	VPINSRW $6, boff+168(BX), X3, X3; \
	VPINSRW $7, boff+196(BX), X3, X3; \
	VPMOVSXWD X3, Y3; \
	VPSUBD Y3, Y2, Y4; \
	VPMULLD Y4, Y4, Y4; \
	VPMOVSXDQ X4, Y5; \
	VEXTRACTI128 $1, Y4, X6; \
	VPMOVSXDQ X6, Y6; \
	VPADDQ Y5, Y0, Y0; \
	VPADDQ Y6, Y1, Y1; \
	VPXOR X3, X3, X3

// func quantized8DistancesRowMajorAVX2(query *int16, vectors unsafe.Pointer, out *int64)
TEXT ·quantized8DistancesRowMajorAVX2(SB), NOSPLIT, $0-24
	MOVQ query+0(FP), AX
	MOVQ vectors+8(FP), BX
	MOVQ out+16(FP), CX

	VPXOR Y0, Y0, Y0
	VPXOR Y1, Y1, Y1
	VPXOR X3, X3, X3

	// Same dimension order as quantizedDistance/centroid scalar path.
	ROWSTEP(10, 10)  // dim 5
	ROWSTEP(12, 12)  // dim 6
	ROWSTEP(4, 4)    // dim 2
	ROWSTEP(0, 0)    // dim 0
	ROWSTEP(14, 14)  // dim 7
	ROWSTEP(16, 16)  // dim 8
	ROWSTEP(24, 24)  // dim 12
	ROWSTEP(2, 2)    // dim 1
	ROWSTEP(6, 6)    // dim 3
	ROWSTEP(8, 8)    // dim 4
	ROWSTEP(18, 18)  // dim 9
	ROWSTEP(20, 20)  // dim 10
	ROWSTEP(22, 22)  // dim 11
	ROWSTEP(26, 26)  // dim 13

	VMOVDQU Y0, 0(CX)
	VMOVDQU Y1, 32(CX)
	VZEROUPPER
	RET
