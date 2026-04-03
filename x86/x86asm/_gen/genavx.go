// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// genavx generates data tables for AVX instructions based on XED data,
// used in x86asm.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/arch/x86/xeddata"
)

var (
	db      *xeddata.Database
	xedPath string
	outFile string
)

func main() {
	log.SetPrefix("genavx: ")
	log.SetFlags(0)

	flag.StringVar(&xedPath, "xedPath", "", "XED datafiles location")
	flag.StringVar(&outFile, "o", "", "output file (stdout if empty)")
	flag.Parse()

	if xedPath == "" {
		xedPath = os.Getenv("XEDPATH")
	}
	if xedPath == "" {
		log.Fatalf("XEDPATH not set and -xedPath not provided")
	}

	var err error
	db, err = xeddata.NewDatabase(xedPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	generate()
}

type instruction struct {
	iclass    string
	opcode    string
	opbyte    string
	opdigit   string // "0"-"7" or ""
	vexL      string // 0, 1, 2
	vexW      string // 0, 1
	vexP      string // 0, 1, 2, 3
	mapSelect string // 0F, 0F38, 0F3A
	evex      bool
	args      []string // ArgType names, in XED (i.e. Intel) order
	ismem     int      // -1 = any, 0 = reg, 1 = mem
	dispScale int      // EVEX scale
	bcstScale int      // EVEX broadcast scale
	memBytes  uint8    // Memory width in bytes
	vsib      bool     // Uses VSIB addressing
}

var registerArgs = map[string]string{
	"GPR32_R()":  "argGPR32_R",
	"GPR64_R()":  "argGPR64_R",
	"VGPR32_R()": "argGPR32_R",
	"VGPR64_R()": "argGPR64_R",
	"VGPR32_N()": "argGPR32_N",
	"VGPR64_N()": "argGPR64_N",
	"GPR32_B()":  "argGPR32_B",
	"GPR64_B()":  "argGPR64_B",
	"VGPR32_B()": "argGPR32_B",
	"VGPR64_B()": "argGPR64_B",

	"XMM_R()":  "argXmm_R",
	"XMM_R3()": "argXmmEvex_R",
	"XMM_N()":  "argXmm_N",
	"XMM_N3()": "argXmmEvex_N",
	"XMM_B()":  "argXmm_B",
	"XMM_B3()": "argXmmEvex_B",
	"XMM_SE()": "argXmm_SE",

	"YMM_R()":  "argYmm_R",
	"YMM_R3()": "argYmmEvex_R",
	"YMM_N()":  "argYmm_N",
	"YMM_N3()": "argYmmEvex_N",
	"YMM_B()":  "argYmm_B",
	"YMM_B3()": "argYmmEvex_B",
	"YMM_SE()": "argYmm_SE",

	"ZMM_R3()": "argZmm_R",
	"ZMM_N3()": "argZmm_N",
	"ZMM_B3()": "argZmm_B",

	"MASK_R()": "argK_R",
	"MASK_N()": "argK_N",
	"MASK_B()": "argK_B",

	"MASKNOT0()": "argKnot0",
}

func generate() {
	var insts []*instruction

	err := xeddata.WalkInsts(xedPath, func(inst *xeddata.Inst) {
		inst.Pattern = xeddata.ExpandStates(db, inst.Pattern)
		pset := xeddata.NewPatternSet(inst.Pattern)

		if inst.HasAttribute("AMDONLY") || inst.Extension == "XOP" {
			return
		}
		if !pset.Is("VEX") && !pset.Is("EVEX") {
			return
		}
		if !strings.HasPrefix(inst.Iclass, "V") && !strings.HasPrefix(inst.Iclass, "K") {
			// Handle only AVX instructions for now.
			return
		}
		if inst.RealOpcode == "N" {
			return
		}

		dec := &instruction{
			iclass: inst.Iclass,
			evex:   pset.Is("EVEX"),
			vsib:   strings.Contains(inst.Iclass, "GATHER") || strings.Contains(inst.Iclass, "SCATTER"),
		}

		dec.opdigit = findOpdigit(pset)
		dec.opbyte = findOpbyte(pset)

		// Parse args
		for _, f := range strings.Fields(inst.Operands) {
			xarg, err := xeddata.NewOperand(db, f)
			if err != nil {
				continue
			}
			if xarg.Action == "" || !xarg.IsVisible() {
				continue
			}

			name := xarg.NameLHS()
			switch name {
			case "IMM0":
				if pset["IMM0SIGNED=1"] {
					dec.args = append(dec.args, "argImm8")
				} else {
					dec.args = append(dec.args, "argImm8u")
				}
			case "REG0", "REG1", "REG2", "REG3":
				rhs := xarg.NameRHS()
				if rhs == "MASK1()" {
					dec.args = append(dec.args, "argKmask")
					continue
				}
				arg := registerArgs[rhs]
				if arg == "" {
					log.Printf("unknown reg: %s", rhs)
					return
				}
				dec.args = append(dec.args, arg)
			case "MEM0":
				dec.args = append(dec.args, "argM")
			}
		}

		dec.vexP = pset.Match(
			"VEX_PREFIX=1", "1", // 66
			"VEX_PREFIX=2", "2", // F2
			"VEX_PREFIX=3", "3") // F3
		if dec.vexP == "" {
			dec.vexP = "0" // None
		}

		dec.mapSelect = pset.Match(
			"MAP=1", "1", // 0F
			"MAP=2", "2", // 0F38
			"MAP=3", "3") // 0F3A

		dec.vexL = pset.Match(
			"VL=0", "0", // 128
			"VL=1", "1", // 256
			"VL=2", "2") // 512
		if dec.vexL == "" {
			dec.vexL = "0"
		}

		dec.vexW = pset.Match(
			"REXW=1", "1")
		if dec.vexW == "" {
			dec.vexW = "0"
		}

		dec.ismem = -1
		if pset.Is("MemOnly") {
			dec.ismem = 1
		} else if pset.Is("RegOnly") {
			dec.ismem = 0
		}

		if strings.Contains(inst.Attributes, "DISP8_") {
			dec.dispScale = evexScale(pset)
			dec.bcstScale = evexBcstScale(pset, inst)
		}

		for _, op := range strings.Fields(inst.Operands) {
			if strings.HasPrefix(op, "MEM") {
				parts := strings.Split(op, ":")
				if len(parts) >= 3 {
					wcode := parts[2]
					sizeStr := db.WidthSize(wcode, xeddata.OpSize64)
					if sizeStr != "" {
						var size int
						fmt.Sscanf(sizeStr, "%d", &size)
						if strings.HasSuffix(sizeStr, "bits") {
							size /= 8
						}
						dec.memBytes = uint8(size)
					}
				}
				break
			}
		}

		insts = append(insts, dec)
	})
	if err != nil {
		log.Fatalf("walk: %v", err)
	}

	printTables(outFile, insts)
}

func findOpdigit(pset xeddata.PatternSet) string {
	reg := pset.Index(
		"REG[0b000]", "REG[0b001]", "REG[0b010]", "REG[0b011]",
		"REG[0b100]", "REG[0b101]", "REG[0b110]", "REG[0b111]",
	)
	if reg != -1 {
		return fmt.Sprintf("%d", reg)
	}
	return ""
}

var opbyteRE = regexp.MustCompile(`0x[0-9A-F]{2}`)

func findOpbyte(pset xeddata.PatternSet) string {
	for k := range pset {
		if opbyteRE.MatchString(k) {
			return k
		}
	}
	return ""
}

// evexScale returns the scaling factor for EVEX compressed displacement
// when broadcasting is not used.
//
// E.g. (in Intel syntax) for "VADDPS ZMM1, ZMM2, [RAX+64]",
// the scale factor is 64, the disp8 value in the encoding is 1.
func evexScale(pset xeddata.PatternSet) int {
	scaleStr := ""
	switch {
	case pset["NELEM_FULL()"], pset["NELEM_FULLMEM()"]:
		scaleStr = pset.Match("VL=0", "N16", "VL=1", "N32", "VL=2", "N64")
	case pset["NELEM_MOVDDUP()"]:
		scaleStr = pset.Match("VL=0", "N8", "VL=1", "N32", "VL=2", "N64")
	case pset["NELEM_HALF()"], pset["NELEM_HALFMEM()"]:
		scaleStr = pset.Match("VL=0", "N8", "VL=1", "N16", "VL=2", "N32")
	case pset["NELEM_QUARTERMEM()"]:
		scaleStr = pset.Match("VL=0", "N4", "VL=1", "N8", "VL=2", "N16")
	case pset["NELEM_EIGHTHMEM()"]:
		scaleStr = pset.Match("VL=0", "N2", "VL=1", "N4", "VL=2", "N8")
	case pset["NELEM_TUPLE2()"]:
		scaleStr = pset.Match("ESIZE_32_BITS()", "N8", "ESIZE_64_BITS()", "N16")
	case pset["NELEM_TUPLE4()"]:
		scaleStr = pset.Match("ESIZE_32_BITS()", "N16", "ESIZE_64_BITS()", "N32")
	case pset["NELEM_TUPLE8()"]:
		scaleStr = "N32"
	case pset["NELEM_MEM128()"], pset["NELEM_TUPLE1_4X()"]:
		scaleStr = "N16"
	}

	if scaleStr == "" {
		scalars := [...]string{
			"NELEM_SCALAR()", "NELEM_GSCAT()", "NELEM_GPR_READER()",
			"NELEM_GPR_READER_BYTE()", "NELEM_GPR_READER_WORD()",
			"NELEM_GPR_WRITER_STORE()", "NELEM_GPR_WRITER_STORE_BYTE()",
			"NELEM_GPR_WRITER_STORE_WORD()", "NELEM_GPR_WRITER_LDOP_D()",
			"NELEM_GPR_WRITER_LDOP_Q()", "NELEM_TUPLE1()", "NELEM_TUPLE1_BYTE()",
			"NELEM_TUPLE1_WORD()", "NELEM_ONE()",
		}
		for _, scalar := range scalars {
			if pset[scalar] {
				scaleStr = pset.Match(
					"ESIZE_8_BITS()", "N1",
					"ESIZE_16_BITS()", "N2",
					"ESIZE_32_BITS()", "N4",
					"ESIZE_64_BITS()", "N8")
				break
			}
		}
	}

	switch scaleStr {
	case "N1":
		return 1
	case "N2":
		return 2
	case "N4":
		return 4
	case "N8":
		return 8
	case "N16":
		return 16
	case "N32":
		return 32
	case "N64":
		return 64
	}
	return 0
}

// evexBcstScale returns the scaling factor for EVEX compressed displacement
// when broadcasting is used (i.e. evex_b == 1). In this mode, the scale
// factor is the element size, not the full vector size.
//
// E.g. (in Intel syntax) for "VADDPS ZMM1, ZMM2, [RAX+4]{1to16}",
// the scale factor is 4, and the disp8 value in the encoding is 1.
func evexBcstScale(pset xeddata.PatternSet, inst *xeddata.Inst) int {
	scaleStr := ""
	switch {
	case pset["NELEM_FULL()"]:
		scaleStr = pset.Match(
			"ESIZE_32_BITS()", "BcstN4",
			"ESIZE_64_BITS()", "BcstN8")
	case pset["NELEM_HALF()"]:
		scaleStr = "BcstN4"
	default:
		if inst.HasAttribute("BROADCAST_ENABLED") {
			scaleStr = pset.Match(
				"ESIZE_32_BITS()", "BcstN4",
				"ESIZE_64_BITS()", "BcstN8")
		}
	}

	switch scaleStr {
	case "BcstN4":
		return 4
	case "BcstN8":
		return 8
	}
	return 0
}

func printTables(outFile string, insts []*instruction) {
	// Group by Map and Opcode Byte.
	// map[string]map[string][]*instruction  // Map -> OpcodeByte -> []Inst
	groups := make(map[string]map[uint8][]*instruction)
	groups["1"] = make(map[uint8][]*instruction)
	groups["2"] = make(map[uint8][]*instruction)
	groups["3"] = make(map[uint8][]*instruction)

	for _, inst := range insts {
		if inst.opbyte == "" {
			continue
		}
		var opbyte uint8
		fmt.Sscanf(inst.opbyte, "0x%02X", &opbyte)
		m := inst.mapSelect
		if m == "" {
			log.Printf("missing map for %s", inst.iclass)
			continue
		}
		groups[m][opbyte] = append(groups[m][opbyte], inst)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `// Code generated by genavx. DO NOT EDIT.

package x86asm

`)

	opsMap := make(map[string]bool)
	for _, inst := range insts {
		opsMap[inst.iclass] = true
	}
	var ops []string
	for op := range opsMap {
		ops = append(ops, op)
	}
	sort.Strings(ops)

	fmt.Fprintf(&buf, "const (\n")
	fmt.Fprintf(&buf, "\t_ Op = iota + maxNonAVXOp\n")
	for _, op := range ops {
		fmt.Fprintf(&buf, "\t%s\n", op)
	}
	fmt.Fprintf(&buf, ")\n\n")

	fmt.Fprintf(&buf, "var avxOpNames = []string{\n")
	for _, op := range ops {
		fmt.Fprintf(&buf, "\t%s: %q,\n", op, op)
	}
	fmt.Fprintf(&buf, "}\n\n")

	if len(ops) > 0 {
		fmt.Fprintf(&buf, "const maxOp = %s\n\n", ops[len(ops)-1])
	}

	vsibOps := make(map[string]bool)
	for _, inst := range insts {
		if inst.vsib {
			vsibOps[inst.iclass] = true
		}
	}
	var vsibOpsList []string
	for op := range vsibOps {
		vsibOpsList = append(vsibOpsList, op)
	}
	sort.Strings(vsibOpsList)

	fmt.Fprintf(&buf, "func isVSIB(op Op) bool {\n")
	fmt.Fprintf(&buf, "\tswitch op {\n")
	if len(vsibOpsList) > 0 {
		fmt.Fprintf(&buf, "\tcase ")
		for i, op := range vsibOpsList {
			if i > 0 {
				fmt.Fprintf(&buf, ", ")
			}
			fmt.Fprintf(&buf, "%s", op)
		}
		fmt.Fprintf(&buf, ":\n\t\treturn true\n")
	}
	fmt.Fprintf(&buf, "\t}\n")
	fmt.Fprintf(&buf, "\treturn false\n")
	fmt.Fprintf(&buf, "}\n\n")
	fmt.Fprintf(&buf, `type avxOptab struct {
	op       Op
	args     [6]argType
	vexP     uint8 // 0=any, 1=none, 2=66, 3=F2, 4=F3
	vexL     uint8 // 0=128, 1=256, 2=512
	vexW     uint8 // 0=W0, 1=W1
	opdigit  int8  // -1 if none
	ismem    int8  // -1 = any, 0 = reg, 1 = mem
	evex     bool
	dispScale int8
	bcstScale int8
	memBytes  uint8
	vsib      bool
}

`)

	emitTable(&buf, "avxMap0F", groups["1"])
	emitTable(&buf, "avxMap0F38", groups["2"])
	emitTable(&buf, "avxMap0F3A", groups["3"])

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("gofmt failed: %v\nsource:\n%s", err, buf.Bytes())
	}

	w := os.Stdout
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			log.Fatalf("create: %v", err)
		}
		defer f.Close()
		w = f
	}
	if _, err := w.Write(src); err != nil {
		log.Fatalf("write: %v", err)
	}
}

func emitTable(buf *bytes.Buffer, name string, m map[uint8][]*instruction) {
	fmt.Fprintf(buf, "var %s = [256][]*avxOptab{\n", name)
	for i := 0; i < 256; i++ {
		list := m[uint8(i)]
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(buf, "\t%d: {\n", i)
		for _, inst := range list {
			var fields []string
			fields = append(fields, fmt.Sprintf("op: %s", inst.iclass))

			if len(inst.args) > 0 {
				fields = append(fields, fmt.Sprintf("args: [6]argType{%s}", strings.Join(inst.args, ", ")))
			}
			if inst.vexP != "0" {
				fields = append(fields, fmt.Sprintf("vexP: %s", inst.vexP))
			}
			if inst.vexL != "0" {
				fields = append(fields, fmt.Sprintf("vexL: %s", inst.vexL))
			}
			if inst.vexW != "0" {
				fields = append(fields, fmt.Sprintf("vexW: %s", inst.vexW))
			}
			if inst.opdigit == "" {
				fields = append(fields, "opdigit: -1")
			} else if inst.opdigit != "0" {
				fields = append(fields, fmt.Sprintf("opdigit: %s", inst.opdigit))
			}
			if inst.ismem != 0 {
				fields = append(fields, fmt.Sprintf("ismem: %d", inst.ismem))
			}
			if inst.evex {
				fields = append(fields, "evex: true")
			}
			if inst.dispScale != 0 {
				fields = append(fields, fmt.Sprintf("dispScale: %d", inst.dispScale))
			}
			if inst.bcstScale != 0 {
				fields = append(fields, fmt.Sprintf("bcstScale: %d", inst.bcstScale))
			}
			if inst.memBytes != 0 {
				fields = append(fields, fmt.Sprintf("memBytes: %d", inst.memBytes))
			}
			if inst.vsib {
				fields = append(fields, "vsib: true")
			}

			fmt.Fprintf(buf, "\t\t{%s},\n", strings.Join(fields, ", "))
		}
		fmt.Fprintf(buf, "\t},\n")
	}
	fmt.Fprintf(buf, "}\n\n")
}
