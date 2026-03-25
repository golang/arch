// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var defaultEncodingTmpl string = `// TODO: implement this.
	// Note: the raw value v is from obj.Prog, starting at bit 0.
	// e.g. If it's a register, then its value is following the ones defined in a.out.go
	//      If it's a constant, then its value is the constant value.
	return 0, false` // The %s will be replaced by the symbol name in the operand initialiser.

// encodingImpls are the known implementations of encoding functions, key is their description, value is their implementation.
type encodingImpl struct {
	name      string
	body      string
	encodedIn string
}

// encodingImpls are the known implementations of encoding functions, key is their description, value is their implementation.
var encodingImpls = map[string]encodingImpl{
	"Check this is a B arrangement": {"encodeArngBCheck", `if v == ARNG_B {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a D arrangement": {"encodeArngDCheck", `if v == ARNG_D {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a H arrangement": {"encodeArngHCheck", `if v == ARNG_H {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a Q arrangement": {"encodeArngQCheck", `if v == ARNG_Q {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a S arrangement": {"encodeArngSCheck", `if v == ARNG_S {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a merging predication": {"encodeMergePredCheck", `if v == PRED_M {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	"Check this is a zeroing predication": {"encodeZeroPredCheck", `if v == PRED_Z {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`For the "Byte and halfword" variant: is the size specifier,
sz	<T>
0	B
1	H
bit range mappings:
sz: [22:23)
`: {"encodeSzByteHalfword", `switch v {
	case ARNG_B:
		return 0, true
	case ARNG_H:
		return 1 << 22, true
	}
	return 0, false`, "enc_sz"},
	`For the "Byte, merging" and "Byte, zeroing" variants: is the size specifier,
size	<T>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeByteMergeZero", `switch v {
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`For the "Halfword, merging" and "Halfword, zeroing" variants: is the size specifier,
size[0]	<T>
0	S
1	D
bit range mappings:
size: [22:23)
`: {"encodeSize0HalfwordMergeZero", `switch v {
	case ARNG_S:
		return 0, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_size0"},
	`For the "Word and doubleword" variant: is the size specifier,
sz	<T>
0	S
1	D
bit range mappings:
sz: [22:23)
`: {"encodeSzWordDoubleword", `switch v {
	case ARNG_S:
		return 0, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_sz"},
	`Is an arrangement specifier,
size	<T>
00	16B
01	8H
10	4S
11	2D
bit range mappings:
size: [22:24)
`: {"encodeSize16B8H4S2D", `switch v {
	case ARNG_16B:
		return 0, true
	case ARNG_8H:
		return 1 << 22, true
	case ARNG_4S:
		return 2 << 22, true
	case ARNG_2D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is an arrangement specifier,
size	<T>
00	RESERVED
01	8H
10	4S
11	2D
bit range mappings:
size: [22:24)
`: {"encodeSize8H4S2D", `switch v {
	case ARNG_8H:
		return 1 << 22, true
	case ARNG_4S:
		return 2 << 22, true
	case ARNG_2D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the name of the destination SIMD&FP register, encoded in the "Vd" field.
bit range mappings:
Vd: [0:5)
`: {"encodeVd", `return v & 31, true`, "enc_Vd"},
	`Is the name of the destination scalable predicate register PN8-PN15, with predicate-as-counter encoding, encoded in the "PNd" field.
bit range mappings:
PNd: [0:3)
`: {"encodePNd", `if v >= 24 && v <= 31 {
		// PN registers starts from 16.
		return v - 24, true
	}
	return 0, false`, "enc_PNd"},
	`Is the name of the destination scalable predicate register, encoded in the "Pd" field.
bit range mappings:
Pd: [0:4)
`: {"encodePd", `return v, true`, "enc_Pd"},
	`Is the name of the destination scalable vector register, encoded in the "Zd" field.
bit range mappings:
Zd: [0:5)
`: {"encodeZd", `return v, true`, "enc_Zd"},
	`Is the name of the first source and destination scalable predicate register, encoded in the "Pdn" field.
bit range mappings:
Pdn: [0:4)
`: {"encodePdnDest", `return v, true`, "enc_Pdn"},
	`Is the name of the first source and destination scalable vector register, encoded in the "Zdn" field.
bit range mappings:
Zdn: [0:5)
`: {"encodeZdnDest", `return v, true`, "enc_Zdn"},
	`Is the name of the first source scalable predicate register, encoded in the "Pn" field.
bit range mappings:
Pn: [5:9)
`: {"encodePn59", `return v << 5, true`, "enc_Pn"},
	`Is the name of the first source scalable vector register, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510", `return v << 5, true`, "enc_Zn"},
	`Is the name of the governing scalable predicate register P0-P7, encoded in the "Pg" field.
bit range mappings:
Pg: [10:13)
`: {"encodePg1013", `if v <= 7 {
		return v << 10, true
	}
	return 0, false`, "enc_Pg"},
	`Is the name of the governing scalable predicate register, encoded in the "Pg" field.
bit range mappings:
Pg: [10:14)
`: {"encodePg1014", `return v << 10, true`, "enc_Pg"},
	`Is the name of the governing scalable predicate register, encoded in the "Pg" field.
bit range mappings:
Pg: [5:9)
`: {"encodePg59", `return v << 5, true`, "enc_Pg"},
	`Is the name of the second source and destination scalable predicate register, encoded in the "Pdm" field.
bit range mappings:
Pdm: [0:4)
`: {"encodePdmDest", `return v, true`, "enc_Pdm"},
	`Is the name of the second source and destination scalable vector register, encoded in the "Zda" field.
bit range mappings:
Zda: [0:5)
`: {"encodeZdaDest", `return v, true`, "enc_Zda"},
	`Is the name of the second source scalable predicate register, encoded in the "Pm" field.
bit range mappings:
Pm: [16:20)
`: {"encodePm1620", `return v << 16, true`, "enc_Pm"},
	`Is the name of the second source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [16:21)
`: {"encodeZm1621", `return v << 16, true`, "enc_Zm"},
	`Is the name of the second source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [5:10)
`: {"encodeZm510V1", `return (v & 31) << 5, true`, "enc_Zm"},
	`Is the name of the source and destination scalable predicate register, encoded in the "Pdn" field.
bit range mappings:
Pdn: [0:4)
`: {"encodePdnSrcDst", `return v, true`, "enc_Pdn"},
	`Is the name of the source and destination scalable vector register, encoded in the "Zdn" field.
bit range mappings:
Zdn: [0:5)
`: {"encodeZdnSrcDst", `return v, true`, "enc_Zdn"},
	`Is the name of the source scalable predicate register, encoded in the "Pm" field.
bit range mappings:
Pm: [5:9)
`: {"encodePm59v1", `return v << 5, true`, "enc_Pm"},
	`Is the name of the source scalable predicate register, encoded in the "Pn" field.
bit range mappings:
Pn: [5:9)
`: {"encodePn59v2", `return v << 5, true`, "enc_Pn"},
	`Is the name of the source scalable vector register, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510Src", `return (v & 31) << 5, true`, "enc_Zn"},
	`Is the name of the third source and destination scalable vector register, encoded in the "Zda" field.
bit range mappings:
Zda: [0:5)
`: {"encodeZda3RdSrcDst", `return v, true`, "enc_Zda"},
	`Is the name of the third source scalable vector register, encoded in the "Za" field.
bit range mappings:
Za: [16:21)
`: {"encodeZa16213Rd", `return v << 16, true`, "enc_Za"},
	`Is the name of the third source scalable vector register, encoded in the "Za" field.
bit range mappings:
Za: [5:10)
`: {"encodeZa5103Rd", `return v << 5, true`, "enc_Za"},
	`Is the name of the third source scalable vector register, encoded in the "Zk" field.
bit range mappings:
Zk: [5:10)
`: {"encodeZk5103Rd", `return v << 5, true`, "enc_Zk"},
	`Is the name of the vector select predicate register P0-P7, encoded in the "Pv" field.
bit range mappings:
Pv: [10:13)
`: {"encodePv1013", `return v << 10, true`, "enc_Pv"},
	`Is the name of the vector select predicate register, encoded in the "Pv" field.
bit range mappings:
Pv: [10:14)
`: {"encodePv1014", `return v << 10, true`, "enc_Pv"},
	`Is the name of the vector select predicate register, encoded in the "Pv" field.
bit range mappings:
Pv: [5:9)
`: {"encodePv59", `return v << 5, true`, "enc_Pv"},
	`Is the predication qualifier,
M	<ZM>
0	Z
1	M
bit range mappings:
M: [16:17)
`: {"encodePredQualM1617", `switch v {
	case PRED_Z:
		return 0, true
	case PRED_M:
		return 1 << 16, true
	}
	return 0, false`, "enc_M"},
	`Is the predication qualifier,
M	<ZM>
0	Z
1	M
bit range mappings:
M: [4:5)
`: {"encodePredQualM45", `switch v {
	case PRED_Z:
		return 0, true
	case PRED_M:
		return 1 << 4, true
	}
	return 0, false`, "enc_M"},
	`Is the size specifier,
size	<T>
00	B
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeBHSD2224", `switch v {
	case ARNG_B:
		return 0 << 22, true
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
00	B
01	H
10	S
11	RESERVED
bit range mappings:
size: [22:24)
`: {"encodeSizeBHS2224", `switch v {
	case ARNG_B:
		return 0 << 22, true
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
00	RESERVED
01	B
10	H
11	S
bit range mappings:
size: [22:24)
`: {"encodeSizeBHS2224Offset1", `switch v {
	case ARNG_B:
		return 1 << 22, true
	case ARNG_H:
		return 2 << 22, true
	case ARNG_S:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [13:15)
`: {"encodeSizeHSD1315", `switch v {
	case ARNG_H:
		return 1 << 13, true
	case ARNG_S:
		return 2 << 13, true
	case ARNG_D:
		return 3 << 13, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [17:19)
`: {"encodeSizeHSD1719", `switch v {
	case ARNG_H:
		return 1 << 17, true
	case ARNG_S:
		return 2 << 17, true
	case ARNG_D:
		return 3 << 17, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeHSD2224", `switch v {
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeHSD2224No00", `switch v {
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<T>
01	H
1x	D
bit range mappings:
size: [22:24)
`: {"encodeSizeHD2224", `switch v {
	case ARNG_H:
		return 1 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<Tb>
00	B
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeTbBHSD2224", `switch v {
	case ARNG_B:
		return 0 << 22, true
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<Tb>
00	RESERVED
01	B
10	H
11	S
bit range mappings:
size: [22:24)
`: {"encodeSizeTbBHS2224", `switch v {
	case ARNG_B:
		return 1 << 22, true
	case ARNG_H:
		return 2 << 22, true
	case ARNG_S:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<Tb>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [22:24)
`: {"encodeSizeTbHSD2224Offset1", `switch v {
	case ARNG_H:
		return 1 << 22, true
	case ARNG_S:
		return 2 << 22, true
	case ARNG_D:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size	<Tb>
01	B
1x	S
bit range mappings:
size: [22:24)
`: {"encodeSizeTbBS2224", `switch v {
	case ARNG_B:
		return 1 << 22, true
	case ARNG_S:
		return 3 << 22, true
	}
	return 0, false`, "enc_size"},
	`Is the size specifier,
size[0]	<T>
0	B
1	H
bit range mappings:
size: [22:23)
`: {"encodeSize0BH2223", `switch v {
	case ARNG_B:
		return 0 << 22, true
	case ARNG_H:
		return 1 << 22, true
	}
	return 0, false`, "enc_size0"},
	`Is the size specifier,
size[0]	<T>
0	S
1	D
bit range mappings:
size: [22:23)
`: {"encodeSize0SD2223", `switch v {
	case ARNG_S:
		return 0 << 22, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_size0"},
	`Is the size specifier,
size[0]	<Tb>
0	B
1	H
bit range mappings:
size: [22:23)
`: {"encodeSize0TbBH2223", `switch v {
	case ARNG_B:
		return 0 << 22, true
	case ARNG_H:
		return 1 << 22, true
	}
	return 0, false`, "enc_size0"},
	`Is the size specifier,
sz	<T>
0	S
1	D
bit range mappings:
sz: [14:15)
`: {"encodeSzSD1415", `switch v {
	case ARNG_S:
		return 0 << 14, true
	case ARNG_D:
		return 1 << 14, true
	}
	return 0, false`, "enc_sz"},
	`Is the size specifier,
sz	<T>
0	S
1	D
bit range mappings:
sz: [17:18)
`: {"encodeSzSD1718", `switch v {
	case ARNG_S:
		return 0 << 17, true
	case ARNG_D:
		return 1 << 17, true
	}
	return 0, false`, "enc_sz"},
	`Is the size specifier,
sz	<T>
0	S
1	D
bit range mappings:
sz: [22:23)
`: {"encodeSzSD2223", `switch v {
	case ARNG_S:
		return 0 << 22, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_sz"},
	`Is the size specifier,
tszh	tszl	<T>
0	00	RESERVED
0	01	B
0	10	H
0	11	RESERVED
1	00	S
1	01	RESERVED
1	1x	RESERVED
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeTszhTszlBHS", `switch v {
	case ARNG_B:
		return 0<<22 | 1<<19, true
	case ARNG_H:
		return 0<<22 | 2<<19, true
	case ARNG_S:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<Tb>
0	00	RESERVED
0	01	H
0	10	S
0	11	RESERVED
1	00	D
1	01	RESERVED
1	1x	RESERVED
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeTszhTszlTbHSD", `switch v {
	case ARNG_H:
		return 0<<22 | 1<<19, true
	case ARNG_S:
		return 0<<22 | 2<<19, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`No-op check, returns true`: {"encodeNoop", `return 0, true`, "enc_NIL"},
	`Is the 32-bit name of the source and destination general-purpose register, encoded in the "Rdn" field.
bit range mappings:
Rdn: [0:5)`: {"encodeWdn05", `if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the 64-bit name of the destination SIMD&FP register, encoded in the "Vd" field.
bit range mappings:
Vd: [0:5)`: {"encodeVd0564", `return v & 31, true`, "enc_Vd"},
	`Is the 64-bit name of the destination general-purpose register, encoded in the "Rd" field.
bit range mappings:
Rd: [0:5)`: {"encodeRd05", `if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rd"},
	`Is the 64-bit name of the first source general-purpose register, encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510", `if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the 64-bit name of the second source general-purpose register, encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)`: {"encodeRm1621", `if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the 64-bit name of the source and destination general-purpose register, encoded in the "Rdn" field.
bit range mappings:
Rdn: [0:5)`: {"encodeXdn05", `if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the name of the source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [5:10)`: {"encodeZm510V2", `return (v & 31) << 5, true`, "enc_Zm"},
	`Is the number [0-30] of the destination general-purpose register or the name ZR (31), encoded in the "Rd" field.
bit range mappings:
Rd: [0:5)`: {"encodeRd05ZR", `if v == REG_RSP {
		return 0, false
	}
	// ZR is just R31
	return v & 31, true`, "enc_Rd"},
	`Is the number [0-30] of the general-purpose source register or the name SP (31), encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510SP", `if v == REG_R31 {
		return 0, false
	}
	if v == REG_RSP {
		return (REG_R31 & 31) << 5, true
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the number [0-30] of the source and destination general-purpose register or the name ZR (31), encoded in the "Rdn" field.
bit range mappings:
Rdn: [0:5)`: {"encodeRdn05ZR", `if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)`: {"encodeRm1621ZR", `if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rm" field.
bit range mappings:
Rm: [5:10)`: {"encodeRm510ZR", `if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 5, true`, "enc_Rm"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510ZR", `if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the number [0-31] of the destination SIMD&FP register, encoded in the "Vd" field.
bit range mappings:
Vd: [0:5)`: {"encodeVd05", `return v & 31, true`, "enc_Vd"},
	`Is the number [0-31] of the source SIMD&FP register, encoded in the "Vm" field.
bit range mappings:
Vm: [5:10)`: {"encodeVm510", `return (v & 31) << 5, true`, "enc_Vm"},
	`Is the number [0-31] of the source SIMD&FP register, encoded in the "Vn" field.
bit range mappings:
Vn: [5:10)`: {"encodeVn510", `return (v & 31) << 5, true`, "enc_Vn"},
	`Is the number [0-31] of the source and destination SIMD&FP register, encoded in the "Vdn" field.
bit range mappings:
Vdn: [0:5)`: {"encodeVdn05", `return v & 31, true`, "enc_Vdn"},
}
