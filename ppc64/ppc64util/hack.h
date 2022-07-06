// Copyright 2021 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file requires gcc and binutils with -mcpu=power10 support.
// ppc64util runs a series of commands like:
//   go run map.go -fmt=asm ../pp64.csv > asm.S
//   powerpc64le-linux-gnu-gcc -c asm.S -mcpu=power10 -mbig
//   powerpc64le-linux-gnu-objdump -d asm.o
// to create the file decode_generated.txt used to verify the disassembler.
//
// Note, the golang disassembler is not expected to support every extended
// mnemonic, but it should support those which frequently show up in object
// files compiled by the golang toolchain.

#define RA 1
#define RB 2
#define RS 3
#define RT 4
#define RC 5
#define RSp 6
#define RTp 8

#define MB 1
#define ME 7
#define NB 2
#define CY 1

#define LEV 1

#define FRBp 2
#define FRAp 4
#define FRTp 6
#define FRSp 8
#define FRT 3
#define FRA 5
#define FRB 7
#define FRC 9
#define FRS 11
#define FLM 8
#define U 3
#define W 0
#define TE 15
#define SP 1
#define S 1
#define DRM 0x7
#define RM 0x3

#define BF 3
#define SH 7

#define XT 33
#define XA 35
#define XB 37
#define XS 39
#define XC 41
#define XAp 36
#define XTp 38
#define XSp 40
#define DM 1
#define SHW 2

#define VRA 1
#define VRB 2
#define VRC 3
#define VRT 4
#define VRS 5
#define SHB 3
#define SIX 1
#define ST 1
#define PS 0
#define MP 1
#define bm 0x45FF
#define N 3

#define AT 7
#define AS 6

#define RMC 3

#define UIM 1
#define DCMX 0x23
#define DCM 0x11
#define DGM 0x11
#define R 1

#define BA 1
#define BB 2
#define BT 3
#define BO 4
#define BI 6
#define BH 0
#define BFA 7
#define FXM 8
#define BC 11

#define L 1
#define EH 1

#define SPR 69
#define BHRBE 69
#define TO 0x11
#define TBR 268
#define CT 2
#define FC 2
#define TH 3
#define WC 1
#define PL 0
#define IH 4
#define RIC 1
#define PRS 1

#define SIM 6
#define IMM 13
#define IMM8 14
#define D 0x80
#define SC 1

#define target_addr 0x690

#define XMSK 0x9
#define YMSK 0x3
#define PMSK 0x2

#define IX 1
#define IMM32 0x1234567
#define Dpfx 0x160032
#define RApfx 0x0
#define Rpfx 1
#define SIpfx 0xFFFFFFFE00010007

// A valid displacement value for the hash check and hash store instructions.
#define offset -128

// These decode as m.fpr* or m.vr*.  This is a matter of preference.  We
// don't support these mnemonics, and I don't think they improve reading
// disassembled code in most cases. so ignore.
//
// Likewise, if you add to this list, add tests to decode.txt to ensure we
// still test these, while ignoring the extended mnemonics which get
// generated.
#define mfvsrd xsrsp
#define mfvsrwz xsrsp
#define mtvsrd xsrsp
#define mtvsrwz xsrsp
#define mtvsrwa xsrsp

// isel BC bit is not decoded like other BC fields.
// A special test case is added to decode.txt to verify this.
// We decode it like other BC fields.
#define isel rldicl


// Likewise, these are obscure book ii instructions with extended mnemonics
// which are almost guaranteed never to show up in go code
#define dcbf add
#define sync xsrsp
#define wait xsrsp
#define rfebb sc

// sync 1,1 is the stncisync extended mnemonic.  Similar to the above, but
// the lwsync/hwsync extended mnemonics are tested in decode.txt
#define sync xsrsp
