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
`: {"encodeZd", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zd"},
	`Is the name of the first source and destination scalable predicate register, encoded in the "Pdn" field.
bit range mappings:
Pdn: [0:4)
`: {"encodePdnDest", `return v, true`, "enc_Pdn"},
	`Is the name of the first source and destination scalable vector register, encoded in the "Zdn" field.
bit range mappings:
Zdn: [0:5)
`: {"encodeZdnDest", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zdn"},
	`Is the name of the first source scalable predicate register, encoded in the "Pn" field.
bit range mappings:
Pn: [5:9)
`: {"encodePn59", `return v << 5, true`, "enc_Pn"},
	`Is the name of the first source scalable vector register, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510V1", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 5, true`, "enc_Zn"},
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
`: {"encodeZdaDest", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zda"},
	`Is the name of the second source scalable predicate register, encoded in the "Pm" field.
bit range mappings:
Pm: [16:20)
`: {"encodePm1620", `return v << 16, true`, "enc_Pm"},
	`Is the name of the second source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [16:21)
`: {"encodeZm1621V2", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`Is the name of the second source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [5:10)
`: {"encodeZm510V1", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v & 31) << 5, true`, "enc_Zm"},
	`Is the name of the source and destination scalable predicate register, encoded in the "Pdn" field.
bit range mappings:
Pdn: [0:4)
`: {"encodePdnSrcDst", `return v, true`, "enc_Pdn"},
	`Is the name of the source and destination scalable vector register, encoded in the "Zdn" field.
bit range mappings:
Zdn: [0:5)
`: {"encodeZdnSrcDst", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zdn"},
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
`: {"encodeZn510Src", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v & 31) << 5, true`, "enc_Zn"},
	`Is the name of the third source and destination scalable vector register, encoded in the "Zda" field.
bit range mappings:
Zda: [0:5)
`: {"encodeZda3RdSrcDst", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zda"},
	`Is the name of the third source scalable vector register, encoded in the "Za" field.
bit range mappings:
Za: [16:21)
`: {"encodeZa16213Rd", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 16, true`, "enc_Za"},
	`Is the name of the third source scalable vector register, encoded in the "Za" field.
bit range mappings:
Za: [5:10)
`: {"encodeZa5103Rd", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 5, true`, "enc_Za"},
	`Is the name of the third source scalable vector register, encoded in the "Zk" field.
bit range mappings:
Zk: [5:10)
`: {"encodeZk5103Rd", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 5, true`, "enc_Zk"},
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
Rdn: [0:5)`: {"encodeWdn05", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the 64-bit name of the destination SIMD&FP register, encoded in the "Vd" field.
bit range mappings:
Vd: [0:5)`: {"encodeVd0564", `return v & 31, true`, "enc_Vd"},
	`Is the 64-bit name of the destination general-purpose register, encoded in the "Rd" field.
bit range mappings:
Rd: [0:5)`: {"encodeRd05", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rd"},
	`Is the 64-bit name of the first source general-purpose register, encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the 64-bit name of the second source general-purpose register, encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)`: {"encodeRm1621V1", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the 64-bit name of the source and destination general-purpose register, encoded in the "Rdn" field.
bit range mappings:
Rdn: [0:5)`: {"encodeXdn05", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the name of the source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [5:10)`: {"encodeZm510V2", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v & 31) << 5, true`, "enc_Zm"},
	`Is the number [0-30] of the destination general-purpose register or the name ZR (31), encoded in the "Rd" field.
bit range mappings:
Rd: [0:5)`: {"encodeRd05ZR", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	// ZR is just R31
	return v & 31, true`, "enc_Rd"},
	`Is the number [0-30] of the general-purpose source register or the name SP (31), encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510SPV1", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_R31 {
		return 0, false
	}
	if v == REG_RSP {
		return (REG_R31 & 31) << 5, true
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the number [0-30] of the source and destination general-purpose register or the name ZR (31), encoded in the "Rdn" field.
bit range mappings:
Rdn: [0:5)`: {"encodeRdn05ZR", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return v & 31, true`, "enc_Rdn"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)`: {"encodeRm1621ZR", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rm" field.
bit range mappings:
Rm: [5:10)`: {"encodeRm510ZR", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 5, true`, "enc_Rm"},
	`Is the number [0-30] of the source general-purpose register or the name ZR (31), encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)`: {"encodeRn510ZR", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
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
	`For the "16-bit to 32-bit" variant: is the immediate index of a pair of 16-bit elements within each 128-bit vector segment, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_16To32Bit", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "16-bit to 64-bit" variant: is the immediate index of a 64-bit group of four 16-bit values within each 128-bit vector segment, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [20:21)
`: {"encodeI1_2021_16To64Bit", `if v > 1 {
		return 0, false
	}
	return v << 20, true`, "enc_i1"},
	`For the "16-bit to 64-bit" variant: is the name of the second source scalable vector register Z0-Z15, encoded in the "Zm" field.
bit range mappings:
Zm: [16:20)
`: {"encodeZm1620_16To64Bit", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 15 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "16-bit" and "32-bit" variants: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm1619_16Bit32Bit", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 7 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "16-bit" variant: is the element index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:23)
i3l: [19:21)
`: {"encodeI3hI3l_1923_16Bit", `if v > 7 {
		return 0, false
	}
	return (v&3)<<19 | (v>>2)<<22, true`, "enc_i3h_i3l"},
	`For the "32-bit" variant: is the element index, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_32Bit", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "32-bit" variant: is the element index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [19:21)
i3l: [11:12)
`: {"encodeI3hI3l_1119_32Bit", `if v > 7 {
		return 0, false
	}
	return (v&1)<<11 | (v>>1)<<19, true`, "enc_i3h_i3l"},
	`For the "32-bit" variant: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm1619_32Bit", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 7 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "64-bit" variant: is the element index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [20:21)
`: {"encodeI1_2021_64Bit", `if v > 1 {
		return 0, false
	}
	return v << 20, true`, "enc_i1"},
	`For the "64-bit" variant: is the element index, in the range 0 to 3, encoded in the "i2h:i2l" fields.
bit range mappings:
i2h: [20:21)
i2l: [11:12)
`: {"encodeI2hI2l_1120_64Bit", `if v > 3 {
		return 0, false
	}
	return (v&1)<<11 | (v>>1)<<20, true`, "enc_i2h_i2l"},
	`For the "64-bit" variant: is the name of the second source scalable vector register Z0-Z15, encoded in the "Zm" field.
bit range mappings:
Zm: [16:20)
`: {"encodeZm1620_64Bit", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 15 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "8-bit to 16-bit" variant: is the immediate index of a pair of 8-bit elements within each 128-bit vector segment, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:23)
i3l: [19:21)
`: {"encodeI3hI3l_1923_8To16Bit", `if v > 7 {
		return 0, false
	}
	return (v&3)<<19 | (v>>2)<<22, true`, "enc_i3h_i3l"},
	`For the "8-bit to 32-bit" variant: is the immediate index of a 32-bit group of four 8-bit values within each 128-bit vector segment, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_8To32Bit", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "8-bit to 32-bit" variant: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm1619_8To32Bit", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 7 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "Double-precision" variant: is the immediate index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [20:21)
`: {"encodeI1_2021_DoublePrecision", `if v > 1 {
		return 0, false
	}
	return v << 20, true`, "enc_i1"},
	`For the "Double-precision" variant: is the name of the second source scalable vector register Z0-Z15, encoded in the "Zm" field.
bit range mappings:
Zm: [16:20)
`: {"encodeZm1620_DoublePrecision", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 15 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "Doubleword" variant: is the optional portion index, in the range 0 to 7, defaulting to 0, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:23)
i3l: [17:19)
`: {"encodeI3hI3l_1722_Doubleword", `if v > 7 {
		return 0, false
	}
	return (v&3)<<17 | (v>>2)<<22, true`, "enc_i3h_i3l"},
	`For the "Half-precision" and "Single-precision" variants: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm1619_HalfSinglePrecision", `if !stripRawZ(&v) {
			return 0, false
		}
		if v > 7 {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`For the "Half-precision" variant: is the immediate index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:23)
i3l: [19:21)
`: {"encodeI3hI3l_1923_HalfPrecision", `if v > 7 {
		return 0, false
	}
	return (v&3)<<19 | (v>>2)<<22, true`, "enc_i3h_i3l"},
	`For the "Halfword" variant: is the optional portion index, in the range 0 to 1, defaulting to 0, encoded in the "i1" field.
bit range mappings:
i1: [17:18)
`: {"encodeI1_1718_Halfword", `if v > 1 {
		return 0, false
	}
	return v << 17, true`, "enc_i1"},
	`For the "Single-precision" variant: is the immediate index, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_SinglePrecision", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "Word" variant: is the optional portion index, in the range 0 to 3, defaulting to 0, encoded in the "i2" field.
bit range mappings:
i2: [17:19)
`: {"encodeI2_1719_Word", `if v > 3 {
		return 0, false
	}
	return v << 17, true`, "enc_i2"},
	`Is the immediate index of a 32-bit group of four 8-bit values within each 128-bit vector segment, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_8BitGroup", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`Is the immediate index of a pair of 16-bit elements within each 128-bit vector segment, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_Pair16Bit", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`Is the immediate index of a pair of 8-bit elements within each 128-bit vector segment, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [19:21)
i3l: [11:12)
`: {"encodeI3hI3l_1119_Pair8Bit", `if v > 7 {
		return 0, false
	}
	return (v&1)<<11 | (v>>1)<<19, true`, "enc_i3h_i3l"},
	`Is the immediate index, in the range 0 to 15, encoded in the "i4h:i4l" fields.
bit range mappings:
i4h: [19:21)
i4l: [10:12)
`: {"encodeI4hI4l_1019", `if v > 15 {
		return 0, false
	}
	return (v&3)<<10 | (v>>2)<<19, true`, "enc_i4h_i4l"},
	`Is the immediate index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [19:21)
i3l: [11:12)
`: {"encodeI3hI3l_1119", `if v > 7 {
		return 0, false
	}
	return (v&1)<<11 | (v>>1)<<19, true`, "enc_i3h_i3l"},
	`Is the immediate index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:23)
i3l: [19:21)
`: {"encodeI3hI3l_1922", `if v > 7 {
		return 0, false
	}
	return (v&3)<<19 | (v>>2)<<22, true`, "enc_i3h_i3l"},
	`Is the immediate index, in the range 0 to one less than the number of elements in 128 bits, encoded in "i1:tsz".
bit range mappings:
i1: [20:21)
tsz: [16:20)
`: {"encodeI1Tsz_Delegate", `// The statement "range 0 to one less than the number of elements in 128 bits"
	// is not possible to handle here, we delegate this to the caller.
	return codeI1Tsz, false`, "enc_i1_tsz"},
	`Is the immediate index, in the range 0 to one less than the number of elements in 512 bits, encoded in "imm2:tsz".
bit range mappings:
imm2: [22:24)
tsz: [16:21)
`: {"encodeImm2Tsz_Delegate", `// The statement "range 0 to one less than the number of elements in 512 bits"
	// is not possible to handle here, we delegate this to the caller.
	return codeImm2Tsz, false`, "enc_imm2_tsz"},
	`Is the name of the first source scalable predicate register PN8-PN15, with predicate-as-counter encoding, encoded in the "PNn" field.
bit range mappings:
PNn: [5:8)
`: {"encodePnN_58", `if v >= 24 && v <= 31 {
		// PN registers starts from 16.
		return (v - 24) << 5, true
	}
	return 0, false`, "enc_PNn"},
	`Is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm_1619_Range0_7V2", `if v <= 7 {
		return v << 16, true
	}
	return 0, false`, "enc_Zm"},
	`Is the portion index, in the range 0 to 3, encoded in the "imm2" field.
bit range mappings:
imm2: [8:10)
`: {"encodeImm2_810", `if v > 3 {
		return 0, false
	}
	return v << 8, true`, "enc_imm2"},
	`Is the size specifier,
tsz	<T>
0000	RESERVED
xxx1	B
xx10	H
x100	S
1000	D
bit range mappings:
tsz: [16:20)
`: {"encodeTsz_1620_SizeSpecifier4", `switch v {
	case ARNG_B:
		return 1 << 16, true
	case ARNG_H:
		return 2 << 16, true
	case ARNG_S:
		return 4 << 16, true
	case ARNG_D:
		return 8 << 16, true
	}
	return 0, false`, "enc_tsz"},

	`Is the size specifier,
tsz	<T>
00000	RESERVED
xxxx1	B
xxx10	H
xx100	S
x1000	D
10000	Q
bit range mappings:
tsz: [16:21)
`: {"encodeTsz_1621_SizeSpecifier5", `switch v {
	case ARNG_B:
		return 1 << 16, true
	case ARNG_H:
		return 2 << 16, true
	case ARNG_S:
		return 4 << 16, true
	case ARNG_D:
		return 8 << 16, true
	case ARNG_Q:
		return 16 << 16, true
	}
	return 0, false`, "enc_tsz"},
	`Check this is a 64-bit scalar register`: {"encodeXCheck", `return 0, true`, "enc_NIL"},
	`Check this is immediate 0.0`: {"encodeFimm0_0_56", `if (v & 0x7FFFFFFF) != 0 {
		return 0, false
	}
	return 0, true`, "enc_NIL"},
	`For the "16-bit" variant: is the element index, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_16bit", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "16-bit" variant: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm_1619_Range0_7V1", `if !stripRawZ(&v) {
			return 0, false
		}
		if v <= 7 {
			return v << 16, true
		}
		return 0, false`, "enc_Zm"},
	`For the "32-bit" variant: is the element index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [20:21)
`: {"encodeI1_2021_32bit", `if v > 1 {
		return 0, false
	}
	return v << 20, true`, "enc_i1"},
	`For the "32-bit" variant: is the name of the second source scalable vector register Z0-Z15, encoded in the "Zm" field.
bit range mappings:
Zm: [16:20)
`: {"encodeZm_1620_Range0_15", `if !stripRawZ(&v) {
			return 0, false
		}
		if v <= 15 {
			return v << 16, true
		}
		return 0, false`, "enc_Zm"},
	`For the "Equal", "Greater than or equal", "Greater than", "Less than or equal", "Less than", and "Not equal" variants: is the signed immediate operand, in the range -16 to 15, encoded in the "imm5" field.
bit range mappings:
imm5: [16:21)
`: {"encodeImm5Signed_1621V2", `if int32(v) >= -16 && int32(v) <= 15 {
		return (v & 31) << 16, true
	}
	return 0, false`, "enc_imm5"},
	`For the "Half-precision" variant: is the index of a Real and Imaginary pair, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [19:21)
`: {"encodeI2_1921_Half", `if v > 3 {
		return 0, false
	}
	return v << 19, true`, "enc_i2"},
	`For the "Half-precision" variant: is the name of the second source scalable vector register Z0-Z7, encoded in the "Zm" field.
bit range mappings:
Zm: [16:19)
`: {"encodeZm_1619_Half", `if !stripRawZ(&v) {
			return 0, false
		}
		if v <= 7 {
			return v << 16, true
		}
		return 0, false`, "enc_Zm"},
	`For the "Higher or same", "Higher", "Lower or same", and "Lower" variants: is the unsigned immediate operand, in the range 0 to 127, encoded in the "imm7" field.
bit range mappings:
imm7: [14:21)
`: {"encodeImm7Unsigned_1421", `if v <= 127 {
		return v << 14, true
	}
	return 0, false`, "enc_imm7"},
	`For the "Single-precision" variant: is the index of a Real and Imaginary pair, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [20:21)
`: {"encodeI1_2021_Single", `if v > 1 {
		return 0, false
	}
	return v << 20, true`, "enc_i1"},
	`For the "Single-precision" variant: is the name of the second source scalable vector register Z0-Z15, encoded in the "Zm" field.
bit range mappings:
Zm: [16:20)
`: {"encodeZm_1620_Single", `if !stripRawZ(&v) {
			return 0, false
		}
		if v <= 15 {
			return v << 16, true
		}
		return 0, false`, "enc_Zm"},
	`Is a 64, 32, 16 or 8-bit bitmask consisting of replicated 2, 4, 8, 16, 32 or 64 bit fields, each field containing a rotated run of non-zero bits, encoded in the "imm13" field.
bit range mappings:
imm13: [5:18)
`: {"encodeImm13_518", `return codeLogicalImmArrEncoding, false`, "enc_imm13"},
	`Is a floating-point immediate value expressible as ±n÷16×2^r, where n and r are integers such that 16 ≤ n ≤ 31 and -3 ≤ r ≤ 4, i.e. a normalized binary floating-point encoding with 1 sign bit, 3-bit exponent, and 4-bit fractional part, encoded in the "imm8" field.
bit range mappings:
imm8: [5:13)
`: {"encodeImm8_513_Fimm", `if v <= 255 {
		return v << 5, true
	}
	return 0, false`, "enc_imm8"},
	`Is a signed immediate in the range -128 to 127, encoded in the "imm8" field.
bit range mappings:
imm8: [5:13)

Is the optional left shift to apply to the immediate, defaulting to LSL #0 and
sh	<shift>
0	LSL #0
1	LSL #8
bit range mappings:
sh: [13:14)
`: {"encodeImm8SignedLsl8", `vi := int32(v)
	if vi >= -128 && vi <= 127 {
		imm8 := uint32(uint8(int8(vi)))
		return (imm8 << 5), true
	}
	if vi&255 == 0 {
		unshifted := vi >> 8
		if unshifted >= -128 && unshifted <= 127 {
			imm8 := uint32(uint8(int8(unshifted)))
			return (imm8 << 5) | (1 << 13), true
		}
	}
	return 0, false`, "enc_imm8"},
	`Is an unsigned immediate in the range 0 to 255, encoded in the "imm8" field.
bit range mappings:
imm8: [5:13)

Is the optional left shift to apply to the immediate, defaulting to LSL #0 and
sh	<shift>
0	LSL #0
1	LSL #8
bit range mappings:
sh: [13:14)
`: {"encodeImm8UnsignedLsl8", `if v <= 255 {
		return v << 5, true
	}
	if v&255 == 0 {
		unshifted := v >> 8
		if unshifted <= 255 {
			return (unshifted << 5) | (1 << 13), true
		}
	}
	return 0, false`, "enc_imm8"},
	`Is the 64-bit name of the destination general-purpose register or stack pointer, encoded in the "Rd" field.
bit range mappings:
Rd: [0:5)
`: {"encodeRd05_SPAllowed", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_R31 {
		return 0, false
	}
	if v == REG_RSP {
		return 31, true
	}
	return v & 31, true`, "enc_Rd"},
	`Is the 64-bit name of the source general-purpose register or stack pointer, encoded in the "Rn" field.
bit range mappings:
Rn: [16:21)
`: {"encodeRn1621_SPAllowed", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_R31 {
		return 0, false
	}
	if v == REG_RSP {
		return 31 << 16, true
	}
	return (v & 31) << 16, true`, "enc_Rn"},
	`Is the const specifier,
rot	<const>
0	#90
1	#270
bit range mappings:
rot: [10:11)
`: {"encodeRot90_270_1011", `switch v {
	case 90:
		return 0, true
	case 270:
		return 1 << 10, true
	}
	return 0, false`, "enc_rot"},
	`Is the const specifier,
rot	<const>
0	#90
1	#270
bit range mappings:
rot: [16:17)
`: {"encodeRot90_270_1617", `switch v {
	case 90:
		return 0, true
	case 270:
		return 1 << 16, true
	}
	return 0, false`, "enc_rot"},
	`Is the const specifier,
rot	<const>
00	#0
01	#90
10	#180
11	#270
bit range mappings:
rot: [10:12)
`: {"encodeRot0_90_180_270_1012", `switch v {
	case 0:
		return 0, true
	case 90:
		return 1 << 10, true
	case 180:
		return 2 << 10, true
	case 270:
		return 3 << 10, true
	}
	return 0, false`, "enc_rot"},
	`Is the const specifier,
rot	<const>
00	#0
01	#90
10	#180
11	#270
bit range mappings:
rot: [13:15)
`: {"encodeRot0_90_180_270_1315", `switch v {
	case 0:
		return 0, true
	case 90:
		return 1 << 13, true
	case 180:
		return 2 << 13, true
	case 270:
		return 3 << 13, true
	}
	return 0, false`, "enc_rot"},
	`Is the first signed immediate operand, in the range -16 to 15, encoded in the "imm5" field.
bit range mappings:
imm5: [5:10)
`: {"encodeImm5Signed_510", `if int32(v) >= -16 && int32(v) <= 15 {
		return (v & 31) << 5, true
	}
	return 0, false`, "enc_imm5"},
	`Is the floating-point immediate value,
i1	<const>
0	#0.0
1	#1.0
bit range mappings:
i1: [5:6)
`: {"encodeFimm0_0_1_0_56", `switch v {
	case 0:
		return 0, true
	case 0x3F800000: // 1.0
		return 1 << 5, true
	}
	return 0, false`, "enc_i1"},
	`Is the floating-point immediate value,
i1	<const>
0	#0.5
1	#1.0
bit range mappings:
i1: [5:6)
`: {"encodeFimm0_5_1_0_56", `switch v {
	case 0x3F000000: // 0.5
		return 0, true
	case 0x3F800000: // 1.0
		return 1 << 5, true
	}
	return 0, false`, "enc_i1"},
	`Is the floating-point immediate value,
i1	<const>
0	#0.5
1	#2.0
bit range mappings:
i1: [5:6)
`: {"encodeFimm0_5_2_0_56", `switch v {
	case 0x3F000000: // 0.5
		return 0, true
	case 0x40000000: // 2.0
		return 1 << 5, true
	}
	return 0, false`, "enc_i1"},
	`Is the immediate shift amount, in the range 0 to number of bits per element minus 1, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [16:19)
tszh: [22:23)
tszl: [19:21)
`: {"encodeShiftTsz1619Range0V1", `return codeShift161919212223, false`, "enc_tszh_tszl_imm3"},
	`Is the immediate shift amount, in the range 0 to number of bits per element minus 1, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [16:19)
tszh: [22:24)
tszl: [19:21)
`: {"encodeShiftTsz1619Range0V2", `return codeShift161919212224, false`, "enc_tszh_tszl_imm3"},
	`Is the immediate shift amount, in the range 0 to number of bits per element minus 1, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [5:8)
tszh: [22:24)
tszl: [8:10)
`: {"encodeShiftTsz58Range0", `return codeShift588102224, false`, "enc_tszh_tszl_imm3"},
	`Is the immediate shift amount, in the range 1 to number of bits per element, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [16:19)
tszh: [22:23)
tszl: [19:21)
`: {"encodeShiftTsz1619Range1V1", `return codeShift161919212223, false`, "enc_tszh_tszl_imm3"},
	`Is the immediate shift amount, in the range 1 to number of bits per element, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [16:19)
tszh: [22:24)
tszl: [19:21)
`: {"encodeShiftTsz1619Range1V2", `return codeShift161919212224, false`, "enc_tszh_tszl_imm3"},
	`Is the immediate shift amount, in the range 1 to number of bits per element, encoded in "tszh:tszl:imm3".
bit range mappings:
imm3: [5:8)
tszh: [22:24)
tszl: [8:10)
`: {"encodeShiftTsz58Range1", `return codeShift588102224, false`, "enc_tszh_tszl_imm3"},
	`Is the name of the governing scalable predicate register, encoded in the "Pg" field.
bit range mappings:
Pg: [16:20)
`: {"encodePg1620", `return v << 16, true`, "enc_Pg"},
	`Is the second signed immediate operand, in the range -16 to 15, encoded in the "imm5b" field.
bit range mappings:
imm5b: [16:21)
`: {"encodeImm5bSigned_1621", `if int32(v) >= -16 && int32(v) <= 15 {
		return (v & 31) << 16, true
	}
	return 0, false`, "enc_imm5b"},
	`Is the signed immediate operand, in the range -128 to 127, encoded in the "imm8" field.
bit range mappings:
imm8: [5:13)
`: {"encodeImm8Signed_513", `if int32(v) >= -128 && int32(v) <= 127 {
		return (v & 255) << 5, true
	}
	return 0, false`, "enc_imm8"},
	`Is the signed immediate operand, in the range -16 to 15, encoded in the "imm5" field.
bit range mappings:
imm5: [16:21)
`: {"encodeImm5Signed_1621V1", `if int32(v) >= -16 && int32(v) <= 15 {
		return (v & 31) << 16, true
	}
	return 0, false`, "enc_imm5"},
	`Is the signed immediate operand, in the range -16 to 15, encoded in the "imm5" field.
bit range mappings:
imm5: [5:10)
`: {"encodeImm5Signed510Unique", `if int32(v) >= -16 && int32(v) <= 15 {
		return (v & 31) << 5, true
	}
	return 0, false`, "enc_imm5"},
	`Is the signed immediate operand, in the range -32 to 31, encoded in the "imm6" field.
bit range mappings:
imm6: [5:11)
`: {"encodeImm6Signed_511", `if int32(v) >= -32 && int32(v) <= 31 {
		return (v & 63) << 5, true
	}
	return 0, false`, "enc_imm6"},
	`Is the size specifier,
imm13	<T>
0xxxxxx0xxxxx	S
0xxxxxx10xxxx	H
0xxxxxx110xxx	B
0xxxxxx1110xx	B
0xxxxxx11110x	B
0xxxxxx11111x	RESERVED
1xxxxxxxxxxxx	D
bit range mappings:
imm13: [5:18)
`: {"encodeSizeImm13NoOp", `return codeNoOp, false`, "enc_imm13"},
	`Is the size specifier,
tszh	tszl	<T>
0	00	RESERVED
0	01	B
0	1x	H
1	xx	S
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeSizeBhsTsz1921", `switch v {
	case ARNG_B:
		return 1 << 19, true
	case ARNG_H:
		return 2 << 19, true
	case ARNG_S:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<T>
0	00	RESERVED
0	01	H
0	1x	S
1	xx	D
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeSizeHsdTsz1921", `switch v {
	case ARNG_H:
		return 1 << 19, true
	case ARNG_S:
		return 2 << 19, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<T>
00	00	RESERVED
00	01	B
00	01	H
01	xx	S
1x	xx	D
bit range mappings:
tszh: [22:24)
tszl: [19:21)
`: {"encodeSizeBhsdTsz1921", `switch v {
	case ARNG_B:
		return 1 << 19, true
	case ARNG_H:
		return 2 << 19, true
	case ARNG_S:
		return 1 << 22, true
	case ARNG_D:
		return 1 << 23, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<T>
00	00	RESERVED
00	01	B
00	1x	H
01	xx	S
1x	xx	D
bit range mappings:
tszh: [22:24)
tszl: [8:10)
`: {"encodeSizeBhsdTsz810", `switch v {
	case ARNG_B:
		return 1 << 8, true
	case ARNG_H:
		return 2 << 8, true
	case ARNG_S:
		return 1 << 22, true
	case ARNG_D:
		return 1 << 23, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<Tb>
0	00	RESERVED
0	01	B
0	1x	H
1	xx	S
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeSizeBhsTsz1921Unique", `switch v {
	case ARNG_B:
		return 1 << 19, true
	case ARNG_H:
		return 2 << 19, true
	case ARNG_S:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the size specifier,
tszh	tszl	<Tb>
0	00	RESERVED
0	01	H
0	1x	S
1	xx	D
bit range mappings:
tszh: [22:23)
tszl: [19:21)
`: {"encodeSizeHsdTsz1921Unique", `switch v {
	case ARNG_H:
		return 1 << 19, true
	case ARNG_S:
		return 2 << 19, true
	case ARNG_D:
		return 1 << 22, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`Is the unsigned immediate operand, in the range 0 to 15, encoded in the "imm4" field.
bit range mappings:
imm4: [16:20)
`: {"encodeImm4Unsigned_1620", `if v <= 15 {
		return v << 16, true
	}
	return 0, false`, "enc_imm4"},
	`Is the unsigned immediate operand, in the range 0 to 255, encoded in the "imm8" field.
bit range mappings:
imm8: [5:13)
`: {"encodeImm8Unsigned_513", `if v <= 255 {
		return v << 5, true
	}
	return 0, false`, "enc_imm8"},
	`Is the unsigned immediate operand, in the range 0 to 255, encoded in the "imm8h:imm8l" fields.
bit range mappings:
imm8h: [16:21)
imm8l: [10:13)
`: {"encodeImm8hImm8l_Unsigned", `if v <= 255 {
		l := v & 7
		h := v >> 3
		return (l << 10) | (h << 16), true
	}
	return 0, false`, "enc_imm8h_imm8l"},
	`Is the unsigned immediate operand, in the range 0 to 7, encoded in the "imm3" field.
bit range mappings:
imm3: [16:19)
`: {"encodeImm3Unsigned_1619", `if v <= 7 {
		return v << 16, true
	}
	return 0, false`, "enc_imm3"},
	`Is the size specifier,
tszh	tszl	<T>
00	00	RESERVED
00	01	B
00	1x	H
01	xx	S
1x	xx	D
bit range mappings:
tszh: [22:24)
tszl: [19:21)
`: {"encodeSizeBhsdTsz1921", `switch v {
	case ARNG_B:
		return 1 << 19, true
	case ARNG_H:
		return 2 << 19, true
	case ARNG_S:
		return 1 << 22, true
	case ARNG_D:
		return 1 << 23, true
	}
	return 0, false`, "enc_tszh_tszl"},
	`For the "Byte" variant: is the vector segment index, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [22:24)
`: {"encodeI22224", `if v > 3 {
		return 0, false
	}
	return v << 22, true`, "enc_i2"},
	`For the "Byte, single register table" variant: is the vector segment index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [23:24)
`: {"encodeI12324B", `if v > 1 {
		return 0, false
	}
	return v << 23, true`, "enc_i1"},
	`For the "Halfword" variant: is the vector segment index, in the range 0 to 7, encoded in the "i3h:i3l" fields.
bit range mappings:
i3h: [22:24)
i3l: [12:13)
`: {"encodeI3224I31213", `if v > 7 {
		return 0, false
	}
	return (v&1)<<12 | (v>>1)<<22, true`, "enc_i3h_i3l"},
	`For the "Halfword, single register table" and "Halfword, two register table" variants: is the vector segment index, in the range 0 to 3, encoded in the "i2" field.
bit range mappings:
i2: [22:24)
`: {"encodeI22224HW", `if v > 3 {
		return 0, false
	}
	return v << 22, true`, "enc_i2"},
	`Is the name of the first destination scalable predicate register, encoded as "Pd" times 2.
bit range mappings:
Pd: [1:4)
`: {"encodePd14", `if v > 14 {
		return 0, false
	}
	if v&1 != 0 {
		return 0, false
	}
	return v, true`, "enc_Pd"},
	`Is the name of the first destination scalable predicate register, encoded in the "Pd" field.
bit range mappings:
Pd: [0:4)
`: {"encodePd04", `return v, true`, "enc_Pd"},
	`Is the name of the first scalable vector register of the source multi-vector group, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510MultiSrc1", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 5, true`, "enc_Zn"},
	`Is the name of the first table vector register, encoded as "Zn".
bit range mappings:
Zn: [5:10)
`: {"encodeZn510Table1", `return v << 5, true`, "enc_Zn"},
	`Is the name of the second destination scalable predicate register, encoded as "Pd" times 2 plus 1.
bit range mappings:
Pd: [1:4)
`: {"encodePd14Plus1", `if v&1 == 0 {
		return 0, false
	}
	return v - 1, true`, "enc_Pd"},
	`Is the name of the second destination scalable predicate register, encoded in the "Pd" field.
bit range mappings:
Pd: [0:4)
`: {"encodePd04Plus1", `// This "second destination" incurs Pd + 1 == v
	return v - 1, true`, "enc_Pd"},
	`Is the name of the second scalable vector register of the source multi-vector group, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510MultiSrc2", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v - 1) << 5, true`, "enc_Zn"},
	`Is the name of the second table vector register, encoded as "Zn" plus 1 modulo 32.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510Table2", `return ((v - 1) & 0x1f) << 5, true`, "enc_Zn"},
	`Is the name of the source scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [16:21)
`: {"encodeZm1621V1", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`Is the name of the table vector register, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510Table3", `return v << 5, true`, "enc_Zn"},
	`Is the portion index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [8:9)
`: {"encodeI189", `if v > 1 {
		return 0, false
	}
	return v << 8, true`, "enc_i1"},
	`Is the vector segment index, in the range 0 to 1, encoded in the "i1" field.
bit range mappings:
i1: [23:24)
`: {"encodeI12324", `if v > 1 {
		return 0, false
	}
	return v << 23, true`, "enc_i1"},
	`Check this is mod amount and is 1
`: {"encodeModAmt1Check", `if v == 1 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod amount and is 2
`: {"encodeModAmt2Check", `if v == 2 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod amount and is 3
`: {"encodeModAmt3Check", `if v == 3 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod amount and is 4
`: {"encodeModAmt4Check", `if v == 4 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod and is LSL
`: {"encodeModLSLCheck", `if v&0b100 != 0 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod and is SXTW
`: {"encodeModSXTWCheck", `if v&0b10 != 0 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Check this is mod and is UXTW
`: {"encodeModUXTWCheck", `if v&0b1 != 0 {
		return 0, true
	}
	return 0, false`, "enc_NIL"},
	`Is the 64-bit name of the general-purpose base register or stack pointer, encoded in the "Rn" field.
bit range mappings:
Rn: [5:10)
`: {"encodeRn510SPV2", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_R31 {
		return 0, false
	}
	if v == REG_RSP {
		return 31 << 5, true
	}
	return (v & 31) << 5, true`, "enc_Rn"},
	`Is the 64-bit name of the general-purpose offset register, encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)
`: {"encodeRm1621V2", `if !checkIsR(v) {
		return 0, false
	}
	if v == REG_RSP {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the index extend and shift specifier,
msz	<mod>
00	[absent]
x1	LSL
10	LSL
bit range mappings:
msz: [10:12)
`: {"encodeMsz1012", `// This does not accept UXTW and SXTW, check that
	if v&0b11 != 0 {
		return 0, false
	}
	// Note: this encoding function's semantic is entailed by its peer that
	// encode <amount>, so just do nothing.
	return codeNoOp, false`, "enc_msz"},
	`Is the index extend and shift specifier,
xs	<mod>
0	UXTW
1	SXTW
bit range mappings:
xs: [14:15)
`: {"encodeXs1415", `if v&0b1 != 0 {
		return 0, true
	} else if v&0b10 != 0 {
		return 1 << 14, true
	}
	return 0, false`, "enc_xs"},
	`Is the index extend and shift specifier,
xs	<mod>
0	UXTW
1	SXTW
bit range mappings:
xs: [22:23)
`: {"encodeXs2223", `if v&0b1 != 0 {
		return 0, true
	} else if v&0b10 != 0 {
		return 1 << 22, true
	}
	return 0, false`, "enc_xs"},
	`Is the index shift amount,
msz	<amount>
00	[absent]
01	#1
10	#2
11	#3
bit range mappings:
msz: [10:12)
`: {"encodeMsz1012Amount", `if v <= 3 {
		return v << 10, true
	}
	return 0, false`, "enc_msz"},
	`Is the name of the base scalable vector register, encoded in the "Zn" field.
bit range mappings:
Zn: [5:10)
`: {"encodeZn510V2", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 5, true`, "enc_Zn"},
	`Is the name of the first scalable vector register to be transferred, encoded in the "Zt" field.
bit range mappings:
Zt: [0:5)
`: {"encodeZt051", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zt"},
	`Is the name of the fourth scalable vector register to be transferred, encoded as "Zt" plus 3 modulo 32.
bit range mappings:
Zt: [0:5)
`: {"encodeZt054", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v - 3) % 32, true`, "enc_Zt"},
	`Is the name of the offset scalable vector register, encoded in the "Zm" field.
bit range mappings:
Zm: [16:21)
`: {"encodeZm1621V3", `if !stripRawZ(&v) {
			return 0, false
		}
		return v << 16, true`, "enc_Zm"},
	`Is the name of the scalable vector register to be transferred, encoded in the "Zt" field.
bit range mappings:
Zt: [0:5)
`: {"encodeZt05", `if !stripRawZ(&v) {
			return 0, false
		}
		return v, true`, "enc_Zt"},
	`Is the name of the second scalable vector register to be transferred, encoded as "Zt" plus 1 modulo 32.
bit range mappings:
Zt: [0:5)
`: {"encodeZt052", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v - 1) % 32, true`, "enc_Zt"},
	`Is the name of the third scalable vector register to be transferred, encoded as "Zt" plus 2 modulo 32.
bit range mappings:
Zt: [0:5)
`: {"encodeZt053", `if !stripRawZ(&v) {
			return 0, false
		}
		return (v - 2) % 32, true`, "enc_Zt"},
	`Is the optional 64-bit name of the general-purpose offset register, defaulting to XZR, encoded in the "Rm" field.
bit range mappings:
Rm: [16:21)
`: {"encodeRm1621XZR", `if v == 0 {
		// absent case, according to the spec this should be ZR (R31)
		return 31, true
	}
	if !checkIsR(v) {
		return 0, false
	}
	return (v & 31) << 16, true`, "enc_Rm"},
	`Is the size specifier,
size	<T>
00	B
01	H
10	S
11	D
bit range mappings:
size: [21:23)
`: {"encodeSize2123V1", `switch v {
	case ARNG_B:
		return 0, true
	case ARNG_H:
		return 1 << 21, true
	case ARNG_S:
		return 2 << 21, true
	case ARNG_D:
		return 3 << 21, true
	default:
		return 0, false
	}`, "enc_size"},
	`Is the size specifier,
size	<T>
00	RESERVED
01	H
10	S
11	D
bit range mappings:
size: [21:23)
`: {"encodeSize2123V2", `switch v {
	case ARNG_H:
		return 1 << 21, true
	case ARNG_S:
		return 2 << 21, true
	case ARNG_D:
		return 3 << 21, true
	}
	return 0, false`, "enc_size"},
	"Check that there is no modifier (UXTW, SXTW, LSL)": {"encodeNoModCheck", "return 0, v == 0", "enc_NIL"},
	"Check that there is no modifier amount":            {"encodeNoAmtCheck", "return 0, v == 0", "enc_NIL"},
}
