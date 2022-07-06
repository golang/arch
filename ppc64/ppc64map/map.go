// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ppc64map constructs the ppc64 opcode map from the instruction set CSV file.
//
// Usage:
//
//	ppc64map [-fmt=format] ppc64.csv
//
// The known output formats are:
//
//	text (default) - print decoding tree in text form
//	decoder - print decoding tables for the ppc64asm package
//	encoder - generate a self-contained file which can be used to encode
//		  go obj.Progs into machine code
//	asm - generate a gnu asm file which can be compiled by gcc containing
//	      all opcodes discovered in ppc64.csv using macro friendly arguments.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	gofmt "go/format"
	asm "golang.org/x/arch/ppc64/ppc64asm"
	"log"
	"math/bits"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

var format = flag.String("fmt", "text", "output format: text, decoder, asm")
var debug = flag.Bool("debug", false, "enable debugging output")

var inputFile string

type isaversion uint32

const (
	// Sort as supersets of each other. Generally speaking, each newer ISA
	// supports a superset of the previous instructions with a few exceptions
	// throughout.
	ISA_P1 isaversion = iota
	ISA_P2
	ISA_PPC
	ISA_V200
	ISA_V201
	ISA_V202
	ISA_V203
	ISA_V205
	ISA_V206
	ISA_V207
	ISA_V30
	ISA_V30B
	ISA_V30C
	ISA_V31
	ISA_V31B
)

var isaToISA = map[string]isaversion{
	"P1":    ISA_P1,
	"P2":    ISA_P2,
	"PPC":   ISA_PPC,
	"v2.00": ISA_V200,
	"v2.01": ISA_V201,
	"v2.02": ISA_V202,
	"v2.03": ISA_V203,
	"v2.05": ISA_V205,
	"v2.06": ISA_V206,
	"v2.07": ISA_V207,
	"v3.0":  ISA_V30,
	"v3.0B": ISA_V30B,
	"v3.0C": ISA_V30C,
	"v3.1":  ISA_V31,
	"v3.1B": ISA_V31B,
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ppc64map [-fmt=format] ppc64.csv\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("ppc64map: ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}

	inputFile = flag.Arg(0)

	var print func(*Prog)
	switch *format {
	default:
		log.Fatalf("unknown output format %q", *format)
	case "text":
		print = printText
	case "decoder":
		print = printDecoder
	case "asm":
		print = printASM
	case "encoder":
		print = printEncoder
	}

	p, err := readCSV(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Parsed %d instruction forms.", len(p.Insts))
	print(p)
}

// readCSV reads the CSV file and returns the corresponding Prog.
// It may print details about problems to standard error using the log package.
func readCSV(file string) (*Prog, error) {
	// Read input.
	// Skip leading blank and # comment lines.
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	csvReader := csv.NewReader(f)
	csvReader.Comment = '#'
	table, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %v", file, err)
	}
	if len(table) == 0 {
		return nil, fmt.Errorf("empty csv input")
	}
	if len(table[0]) < 4 {
		return nil, fmt.Errorf("csv too narrow: need at least four columns")
	}

	p := &Prog{}
	for _, row := range table {
		add(p, row[0], row[1], row[2], row[3])
	}
	return p, nil
}

type Prog struct {
	Insts     []Inst
	OpRanges  map[string]string
	nextOrder int // Next position value (used for Insts[x].order)
}

type Field struct {
	Name          string
	BitFields     asm.BitFields
	BitFieldNames []string
	Type          asm.ArgType
	Shift         uint8
}

func (f Field) String() string {
	return fmt.Sprintf("%v(%s%v)", f.Type, f.Name, f.BitFields)
}

type Inst struct {
	Text      string
	Encoding  string
	Op        string
	Mask      uint32
	Value     uint32
	DontCare  uint32
	SMask     uint32 // The opcode Mask of the suffix word
	SValue    uint32 // Likewise for the Value
	SDontCare uint32 // Likewise for the DontCare bits
	Fields    []Field
	Words     int // Number of words instruction encodes to.
	Isa       isaversion
	memOp     bool // Is this a memory operation?
	memOpX    bool // Is this an x-form memory operation?
	memOpSt   bool // Is this a store memory operations?
	order     int  // Position in pp64.csv.
}

func (i Inst) String() string {
	return fmt.Sprintf("%s (%s) %08x/%08x[%08x] %v (%s)", i.Op, i.Encoding, i.Value, i.Mask, i.DontCare, i.Fields, i.Text)
}

type Arg struct {
	Name string
	Bits int8
	Offs int8
	// Instruction word position.  0 for single word instructions (all < ISA 3.1 insn)
	// For prefixed instructions, 0 for the prefix word, 1 for the second insn word.
	Word int8
}

func (a Arg) String() string {
	return fmt.Sprintf("%s[%d:%d]", a.Name, a.Offs, a.Offs+a.Bits-1)
}

func (a Arg) Maximum() int {
	return 1<<uint8(a.Bits) - 1
}

func (a Arg) BitMask() uint32 {
	return uint32(a.Maximum()) << a.Shift()
}

func (a Arg) Shift() uint8 {
	return uint8(32 - a.Offs - a.Bits)
}

type Args []Arg

func (as Args) String() string {
	ss := make([]string, len(as))
	for i := range as {
		ss[i] = as[i].String()
	}
	return strings.Join(ss, "|")
}

func (as Args) Find(name string) int {
	for i := range as {
		if as[i].Name == name {
			return i
		}
	}
	return -1
}

func (as *Args) Append(a Arg) {
	*as = append(*as, a)
}

func (as *Args) Delete(i int) {
	*as = append((*as)[:i], (*as)[i+1:]...)
}

func (as Args) Clone() Args {
	return append(Args{}, as...)
}

func (a Arg) isDontCare() bool {
	return a.Name[0] == '/' && a.Name == strings.Repeat("/", len(a.Name))
}

type instArray []Inst

func (i instArray) Len() int {
	return len(i)
}

func (i instArray) Swap(j, k int) {
	i[j], i[k] = i[k], i[j]
}

// Sort by decreasing number of mask bits to ensure extended mnemonics
// are always found first when scanning the table.
func (i instArray) Less(j, k int) bool {
	return bits.OnesCount32(i[j].Mask) > bits.OnesCount32(i[k].Mask)
}

// Split the string encoding into an Args. The encoding string loosely matches the regex
// (arg@bitpos|)+
func parseFields(encoding, text string, word int8) Args {
	var err error
	var args Args

	fields := strings.Split(encoding, "|")

	for i, f := range fields {
		name, off := "", -1
		if f == "" {
			off = 32
			if i == 0 || i != len(fields)-1 {
				fmt.Fprintf(os.Stderr, "%s: wrong %d-th encoding field: %q\n", text, i, f)
				panic("Invalid encoding entry.")
			}
		} else {
			j := strings.Index(f, "@")
			if j < 0 {
				fmt.Fprintf(os.Stderr, "%s: wrong %d-th encoding field: %q\n", text, i, f)
				panic("Invalid encoding entry.")
				continue
			}
			k := strings.Index(f[j+1:], " ")
			if k >= 0 {
				if strings.HasSuffix(f[j+1:], " 31") {
					f = f[:len(f)-3]
				}
			}
			off, err = strconv.Atoi(f[j+1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "err for: %s has: %s for %s\n", f[:j], err, f[j+1:])
			}
			name = f[:j]
		}
		if len(args) > 0 {
			args[len(args)-1].Bits += int8(off)
		}
		if name != "" {
			arg := Arg{Name: name, Offs: int8(off), Bits: int8(-off), Word: word}
			args.Append(arg)
		}
	}

	return args
}

// Compute the Mask (usually Opcode + secondary Opcode bitfields),
// the Value (the expected value under the mask), and
// reserved bits (i.e the // fields which should be set to 0)
func computeMaskValueReserved(args Args, text string) (mask, value, reserved uint32) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		v, err := strconv.Atoi(arg.Name)
		switch {
		case err == nil: // is a numbered field
			if v < 0 || v > arg.Maximum() {
				fmt.Fprintf(os.Stderr, "%s: field %s value (%d) is out of range (%d-bit)\n", text, arg, v, arg.Bits)
			}
			mask |= arg.BitMask()
			value |= uint32(v) << arg.Shift()
			args.Delete(i)
			i--
		case arg.Name[0] == '/': // is don't care
			if arg.Name != strings.Repeat("/", len(arg.Name)) {
				log.Fatalf("%s: arg %v named like a don't care bit, but it's not", text, arg)
			}
			reserved |= arg.BitMask()
			args.Delete(i)
			i--
		default:
			continue
		}
	}

	// rename duplicated fields (e.g. 30@0|RS@6|RA@11|sh@16|mb@21|0@27|sh@30|Rc@31|)
	// but only support two duplicated fields
	for i := 1; i < len(args); i++ {
		if args[:i].Find(args[i].Name) >= 0 {
			args[i].Name += "2"
		}
		if args[:i].Find(args[i].Name) >= 0 {
			log.Fatalf("%s: more than one duplicated fields: %s", text, args)
		}
	}

	// sanity checks
	if mask&reserved != 0 {
		log.Fatalf("%s: mask (%08x) and don't care (%08x) collide", text, mask, reserved)
	}
	if value&^mask != 0 {
		log.Fatalf("%s: value (%08x) out of range of mask (%08x)", text, value, mask)
	}

	var argMask uint32
	for _, arg := range args {
		if arg.Bits <= 0 || arg.Bits > 32 || arg.Offs > 31 || arg.Offs <= 0 {
			log.Fatalf("%s: arg %v has wrong bit field spec", text, arg)
		}
		if mask&arg.BitMask() != 0 {
			log.Fatalf("%s: mask (%08x) intersect with arg %v", text, mask, arg)
		}
		if argMask&arg.BitMask() != 0 {
			log.Fatalf("%s: arg %v overlap with other args %v", text, arg, args)
		}
		argMask |= arg.BitMask()
	}
	if 1<<32-1 != mask|reserved|argMask {
		log.Fatalf("%s: args %v fail to cover all 32 bits", text, args)
	}

	return
}

// Parse a row from the CSV describing the instructions, and place the
// detected instructions into p. One entry may generate multiple intruction
// entries as each extended mnemonic listed in text is treated like a unique
// instruction.
func add(p *Prog, text, mnemonics, encoding, isa string) {
	// Parse encoding, building size and offset of each field.
	// The first field in the encoding is the smallest offset.
	// And note the MSB is bit 0, not bit 31.
	// Example: "31@0|RS@6|RA@11|///@16|26@21|Rc@31|"
	var args, pargs Args
	var pmask, pvalue, presv, resv uint32
	iword := int8(0)
	ispfx := false

	isaLevel, fnd := isaToISA[isa]
	if !fnd {
		log.Fatalf("%s: ISA level '%s' is unknown\n", text, isa)
		return
	}

	// Is this a prefixed instruction?
	if encoding[0] == ',' {
		pfields := strings.Split(encoding, ",")[1:]

		if len(pfields) != 2 {
			log.Fatalf("%s: Prefixed instruction must be 2 words long.\n", text)
			return
		}
		pargs = parseFields(pfields[0], text, iword)
		pmask, pvalue, presv = computeMaskValueReserved(pargs, text)
		// Move to next instruction word
		iword++
		encoding = pfields[1]
		ispfx = true
	}

	args = parseFields(encoding, text, iword)
	mask, value, dontCare := computeMaskValueReserved(args, text)

	if ispfx {
		args = append(args, pargs...)
	}

	// split mnemonics into individual instructions
	// example: "b target_addr (AA=0 LK=0)|ba target_addr (AA=1 LK=0)|bl target_addr (AA=0 LK=1)|bla target_addr (AA=1 LK=1)"
	insts := strings.Split(categoryRe.ReplaceAllString(mnemonics, ""), "|")
	foundInst := []Inst{}
	for _, inst := range insts {
		value, mask := value, mask
		pvalue, pmask := pvalue, pmask
		args := args.Clone()
		if inst == "" {
			continue
		}
		// amend mask and value
		parts := instRe.FindStringSubmatch(inst)
		if parts == nil {
			log.Fatalf("%v couldn't match %s", instRe, inst)
		}
		conds := condRe.FindAllStringSubmatch(parts[2], -1)
		isPCRel := true
		for _, cond := range conds {
			i := args.Find(cond[1])
			v, _ := strconv.ParseInt(cond[2], 16, 32) // the regular expression has checked the number format
			if i < 0 {
				log.Fatalf("%s: %s don't contain arg %s used in %s", text, args, cond[1], inst)
			}
			if cond[1] == "AA" && v == 1 {
				isPCRel = false
			}
			mask |= args[i].BitMask()
			value |= uint32(v) << args[i].Shift()
			args.Delete(i)
		}
		inst := Inst{Text: text, Encoding: parts[1], Value: value, Mask: mask, DontCare: dontCare}
		if ispfx {
			inst = Inst{Text: text, Encoding: parts[1], Value: pvalue, Mask: pmask, DontCare: presv, SValue: value, SMask: mask, SDontCare: resv}
		}

		// order inst.Args according to mnemonics order
		for i, opr := range operandRe.FindAllString(parts[1], -1) {
			if i == 0 { // operation
				inst.Op = opr
				continue
			}
			field := Field{Name: opr}
			typ := asm.TypeUnknown
			var shift uint8
			opr2 := ""
			opr3 := ""
			switch opr {
			case "target_addr":
				shift = 2
				if isPCRel {
					typ = asm.TypePCRel
				} else {
					typ = asm.TypeLabel
				}
				if args.Find("LI") >= 0 {
					opr = "LI"
				} else {
					opr = "BD"
				}

			case "offset":
				switch inst.Op {
				// These encode a 6 bit displacement in the format of an X-form opcode.
				// Allowable displaments are -8 to -8*64 in 8B increments.
				case "hashchk", "hashchkp", "hashst", "hashstp":
					typ = asm.TypeNegOffset
					opr = "DX"
					opr2 = "D"
					shift = 3

				}

			case "XMSK", "YMSK", "PMSK", "IX", "BHRBE":
				typ = asm.TypeImmUnsigned

			case "IMM32":
				typ = asm.TypeImmUnsigned
				opr = "imm0"
				opr2 = "imm1"

			// Handle these cases specially. Note IMM is used on
			// prefixed MMA instructions as a bitmask. Usually, it is a signed value.
			case "R", "UIM", "IMM":
				if ispfx {
					typ = asm.TypeImmUnsigned
					break
				}
				fallthrough

			case "UI", "BO", "BH", "TH", "LEV", "NB", "L", "TO", "FXM", "FC", "U", "W", "FLM", "IMM8", "RIC", "PRS", "SHB", "SHW", "ST", "SIX", "PS", "DCM", "DGM", "RMC", "SP", "S", "DM", "CT", "EH", "E", "MO", "WC", "A", "IH", "OC", "DUI", "DUIS", "CY", "SC", "PL", "MP", "N", "DRM", "RM":
				typ = asm.TypeImmUnsigned
				if i := args.Find(opr); i < 0 {
					log.Printf("coerce to D: %s: couldn't find extended field %s in %s", text, opr, args)
					opr = "D"
				}
			case "bm":
				opr = "b0"
				opr2 = "b1"
				opr3 = "b2"
				typ = asm.TypeImmUnsigned

			case "SH":
				typ = asm.TypeImmUnsigned
				if args.Find("sh2") >= 0 { // sh2 || sh
					opr = "sh2"
					opr2 = "sh"
				}
			case "MB", "ME":
				typ = asm.TypeImmUnsigned
				if n := strings.ToLower(opr); args.Find(n) >= 0 {
					opr = n // xx[5] || xx[0:4]
				}
			case "SI", "SIM", "TE":
				if ispfx {
					typ = asm.TypeImmSigned
					opr = "si0"
					opr2 = "si1"
					break
				}
				typ = asm.TypeImmSigned
				if i := args.Find(opr); i < 0 {
					opr = "D"
				}
			case "DCMX":
				typ = asm.TypeImmUnsigned
				// Some instructions encode this consecutively.
				if i := args.Find(opr); i >= 0 {
					break
				}
				typ = asm.TypeImmUnsigned
				opr = "dc"
				opr2 = "dm"
				opr3 = "dx"
			case "DS":
				typ = asm.TypeOffset
				shift = 2
			case "DQ":
				typ = asm.TypeOffset
				shift = 4
			case "D":
				if ispfx {
					typ = asm.TypeOffset
					opr = "d0"
					opr2 = "d1"
					break
				}
				if i := args.Find(opr); i >= 0 {
					typ = asm.TypeOffset
					break
				}
				if i := args.Find("UI"); i >= 0 {
					typ = asm.TypeImmUnsigned
					opr = "UI"
					break
				}
				if i := args.Find("SI"); i >= 0 {
					typ = asm.TypeImmSigned
					opr = "SI"
					break
				}
				if i := args.Find("d0"); i >= 0 {
					typ = asm.TypeImmSigned
					// DX-form
					opr = "d0"
					opr2 = "d1"
					opr3 = "d2"
				}
			case "RA", "RB", "RC", "RS", "RSp", "RT", "RTp":
				typ = asm.TypeReg
			case "BT", "BA", "BB", "BC", "BI":
				if strings.HasPrefix(inst.Op, "mtfs") {
					// mtfsb[01] instructions use BT, but they specify fields in the fpscr.
					typ = asm.TypeImmUnsigned
				} else {
					typ = asm.TypeCondRegBit
				}
			case "BF", "BFA":
				if strings.HasPrefix(inst.Op, "mtfs") {
					// mtfsfi[.] instructions use BF, but they specify fields in the fpscr.
					typ = asm.TypeImmUnsigned
				} else {
					typ = asm.TypeCondRegField
				}
			case "FRA", "FRB", "FRBp", "FRC", "FRS", "FRSp", "FRT", "FRTp", "FRAp":
				typ = asm.TypeFPReg
			case "XA", "XB", "XC", "XS", "XT": // 5-bit, split field
				typ = asm.TypeVecSReg
				opr2 = opr[1:]
				opr = opr[1:] + "X"
			case "XTp", "XSp": // 5-bit, split field
				//XTp encodes 5 bits, VSR is XT*32 + TP<<1
				typ = asm.TypeVecSpReg
				opr2 = opr[1:2] + "p"
				opr = opr[1:2] + "X"

			case "XAp":
				// XAp in MMA encodes a regular VSR, but is only valid
				// if it is even, and does not overlap the accumulator.
				typ = asm.TypeVecSReg
				opr2 = opr[1:2] + "p"
				opr = opr[1:2] + "X"

			case "AT", "AS":
				typ = asm.TypeMMAReg

			case "VRA", "VRB", "VRC", "VRS", "VRT":
				typ = asm.TypeVecReg

			case "SPR", "TBR":
				typ = asm.TypeSpReg
				if n := strings.ToLower(opr); n != opr && args.Find(n) >= 0 {
					opr = n // spr[5:9] || spr[0:4]
				}
			}
			if typ == asm.TypeUnknown {
				log.Fatalf("%s %s unknown type for opr %s", text, inst, opr)
			}
			field.Type = typ
			field.Shift = shift
			var f1, f2, f3 asm.BitField
			switch {
			case opr3 != "":
				b0 := args.Find(opr)
				b1 := args.Find(opr2)
				b2 := args.Find(opr3)
				f1.Offs, f1.Bits, f1.Word = uint8(args[b0].Offs), uint8(args[b0].Bits), uint8(args[b0].Word)
				f2.Offs, f2.Bits, f2.Word = uint8(args[b1].Offs), uint8(args[b1].Bits), uint8(args[b1].Word)
				f3.Offs, f3.Bits, f3.Word = uint8(args[b2].Offs), uint8(args[b2].Bits), uint8(args[b2].Word)

			case opr2 != "":
				ext := args.Find(opr)
				if ext < 0 {
					log.Fatalf("%s: couldn't find extended field %s in %s", text, opr, args)
				}
				f1.Offs, f1.Bits, f1.Word = uint8(args[ext].Offs), uint8(args[ext].Bits), uint8(args[ext].Word)
				base := args.Find(opr2)
				if base < 0 {
					log.Fatalf("%s: couldn't find base field %s in %s", text, opr2, args)
				}
				f2.Offs, f2.Bits, f2.Word = uint8(args[base].Offs), uint8(args[base].Bits), uint8(args[base].Word)
			case opr == "mb", opr == "me": // xx[5] || xx[0:4]
				i := args.Find(opr)
				if i < 0 {
					log.Fatalf("%s: couldn't find special 'm[be]' field for %s in %s", text, opr, args)
				}
				f1.Offs, f1.Bits, f1.Word = uint8(args[i].Offs+args[i].Bits)-1, 1, uint8(args[i].Word)
				f2.Offs, f2.Bits, f2.Word = uint8(args[i].Offs), uint8(args[i].Bits)-1, uint8(args[i].Word)
			case opr == "spr", opr == "tbr", opr == "tmr", opr == "dcr": // spr[5:9] || spr[0:4]
				i := args.Find(opr)
				if i < 0 {
					log.Fatalf("%s: couldn't find special 'spr' field for %s in %s", text, opr, args)
				}
				if args[i].Bits != 10 {
					log.Fatalf("%s: special 'spr' field is not 10-bit: %s", text, args)
				}
				f1.Offs, f1.Bits, f2.Word = uint8(args[i].Offs)+5, 5, uint8(args[i].Word)
				f2.Offs, f2.Bits, f2.Word = uint8(args[i].Offs), 5, uint8(args[i].Word)
			default:
				i := args.Find(opr)
				if i < 0 {
					log.Fatalf("%s: couldn't find %s in %s", text, opr, args)
				}
				f1.Offs, f1.Bits, f1.Word = uint8(args[i].Offs), uint8(args[i].Bits), uint8(args[i].Word)
			}
			field.BitFields.Append(f1)
			field.BitFieldNames = append(field.BitFieldNames, opr)
			if f2.Bits > 0 {
				field.BitFields.Append(f2)
				field.BitFieldNames = append(field.BitFieldNames, opr2)
			}
			if f3.Bits > 0 {
				field.BitFields.Append(f3)
				field.BitFieldNames = append(field.BitFieldNames, opr3)
			}
			inst.Fields = append(inst.Fields, field)
		}
		if *debug {
			fmt.Printf("%v\n", inst)
		}
		inst.Isa = isaLevel
		inst.memOp = hasMemoryArg(&inst)
		inst.memOpX = inst.memOp && inst.Op[len(inst.Op)-1] == 'x'
		inst.memOpSt = inst.memOp && strings.Contains(inst.Text, "Store")
		inst.Words = 1
		inst.order = p.nextOrder
		p.nextOrder++
		if ispfx {
			inst.Words = 2
		}
		foundInst = append(foundInst, inst)
	}

	// Sort mnemonics by bitcount.  This ensures more specific mnemonics are picked
	// up before generic ones (e.g li vs addi, or cmpld/cmplw vs cmpl)
	sort.Sort(instArray(foundInst))

	p.Insts = append(p.Insts, foundInst...)
}

// condRegexp is a regular expression that matches condition in mnemonics (e.g. "AA=1")
const condRegexp = `\s*([[:alpha:]]+)=([0-9a-f]+)\s*`

// condRe matches condition in mnemonics (e.g. "AA=1")
var condRe = regexp.MustCompile(condRegexp)

// instRe matches instruction with potentially multiple conditions in mnemonics
var instRe = regexp.MustCompile(`^(.*?)\s?(\((` + condRegexp + `)+\))?$`)

// categoryRe matches intruction category notices in mnemonics
var categoryRe = regexp.MustCompile(`(\s*\[Category:[^]]*\]\s*)|(\s*\[Co-requisite[^]]*\]\s*)|(\s*\(\s*0[Xx][[0-9A-Fa-f_]{9}\s*\)\s*)`)

// operandRe matches each operand (including opcode) in instruction mnemonics
var operandRe = regexp.MustCompile(`([[:alpha:]][[:alnum:]_]*\.?)`)

// printText implements the -fmt=text mode, which is not implemented (yet?).
func printText(p *Prog) {
	log.Fatal("-fmt=text not implemented")
}

// Some ISA instructions look like memory ops, but are not.
var isNotMemopMap = map[string]bool{
	"lxvkq": true,
	"lvsl":  true,
	"lvsr":  true,
}

// Some ISA instructions are memops, but are not described like "Load ..." or "Store ..."
var isMemopMap = map[string]bool{
	"hashst":   true,
	"hashstp":  true,
	"hashchk":  true,
	"hashchkp": true,
}

// Does this instruction contain a memory argument (e.g x-form load or d-form store)
func hasMemoryArg(insn *Inst) bool {
	return ((strings.HasPrefix(insn.Text, "Load") || strings.HasPrefix(insn.Text, "Store") ||
		strings.HasPrefix(insn.Text, "Prefixed Load") || strings.HasPrefix(insn.Text, "Prefixed Store")) && !isNotMemopMap[insn.Op]) ||
		isMemopMap[insn.Op]
}

// Generate a function which takes an obj.Proj and convert it into
// machine code in the supplied buffer. These functions are used
// by asm9.go.
func insnEncFuncStr(insn *Inst, firstName [2]string) string {
	buf := new(bytes.Buffer)
	// Argument packing order.
	// Note, if a2 is not a register type, it is skipped.
	argOrder := []string{
		"p.To",               // a6
		"p.From",             // a1
		"p",                  // a2
		"p.RestArgs[0].Addr", // a3
		"p.RestArgs[1].Addr", // a4
		"p.RestArgs[2].Addr", // a5
	}
	if len(insn.Fields) > len(argOrder) {
		log.Fatalf("cannot handle %v. Only %d args supported.", insn, len(argOrder))
	}

	// Does this field require an obj.Addr.Offset?
	isImmediate := func(t asm.ArgType) bool {
		return t == asm.TypeImmUnsigned || t == asm.TypeSpReg || t == asm.TypeImmSigned || t == asm.TypeOffset || t == asm.TypeNegOffset
	}

	if insn.memOp {
		// Swap to/from arguments if we are generating
		// for a store operation.
		if insn.memOpSt {
			// Otherwise, order first three args as: p.From, p.To, p.To
			argOrder[0], argOrder[1] = argOrder[1], argOrder[0]
		}
		argOrder[2] = argOrder[1] // p.Reg is either an Index or Offset (X or D-form)
	} else if len(insn.Fields) > 2 && isImmediate(insn.Fields[2].Type) {
		// Delete the a2 argument if it is not a register type.
		argOrder = append(argOrder[0:2], argOrder[3:]...)
	}

	fmt.Fprintf(buf, "// %s\n", insn.Encoding)
	fmt.Fprintf(buf, "func type_%s(c *ctxt9, p *obj.Prog, t *Optab, out *[5]uint32) {\n", insn.Op)
	if insn.Words > 1 {
		fmt.Fprintf(buf, "o0 := GenPfxOpcodes[p.As - A%s]\n", firstName[1])
	}
	fmt.Fprintf(buf, "o%d := GenOpcodes[p.As - A%s]\n", insn.Words-1, firstName[0])

	errCheck := ""
	for j, atype := range insn.Fields {
		itype := ".Reg"
		if isImmediate(atype.Type) {
			itype = ".Offset"
		} else if insn.memOpX && atype.Name == "RA" {
			// X-form memory operations encode RA as the index register of memory type arg.
			itype = ".Index"
		}

		bitPos := uint64(0)
		// VecSpReg is encoded as an even numbered VSR. It is implicitly shifted by 1.
		if atype.Type == asm.TypeVecSpReg {
			bitPos += 1
		}
		// Count the total number of bits to work backwards when shifting
		for _, f := range atype.BitFields {
			bitPos += uint64(f.Bits)
		}
		// Adjust for any shifting (e.g DQ/DS shifted instructions)
		bitPos += uint64(atype.Shift)
		bits := bitPos

		// Generate code to twirl the respective bits into the correct position, and mask off extras.
		for i, f := range atype.BitFields {
			bitPos -= uint64(f.Bits)
			argStr := argOrder[j] + itype
			if bitPos != 0 {
				argStr = fmt.Sprintf("(%s>>%d)", argStr, bitPos)
			}
			mask := (1 << uint64(f.Bits)) - 1
			shift := 32 - uint64(f.Offs) - uint64(f.Bits)
			fmt.Fprintf(buf, "o%d |= uint32(%s&0x%x)<<%d // %s\n", f.Word, argStr, mask, shift, atype.BitFieldNames[i])
		}

		// Generate a check to verify shifted inputs satisfy their constraints.
		// For historical reasons this is not needed for 16 bit values shifted by 16. (i.e SI/UI constants in addis/xoris)
		if atype.Type != asm.TypeNegOffset && atype.Shift != 0 && atype.Shift != 16 && bits != 32 {
			arg := argOrder[j] + itype
			mod := (1 << atype.Shift) - 1
			errCheck += fmt.Sprintf("if %s & 0x%x != 0 {\n", arg, mod)
			errCheck += fmt.Sprintf("c.ctxt.Diag(\"Constant 0x%%x (%%d) is not a multiple of %d\\n%%v\",%s,%s,p)\n", mod+1, arg, arg)
			errCheck += fmt.Sprintf("}\n")
		}
		// NegOffset requires a stronger offset check
		if atype.Type == asm.TypeNegOffset {
			arg := argOrder[j] + itype
			mask := -1 << (atype.BitFields.NumBits() + int(atype.Shift))
			maskl := mask // Sign bits are implied in this type.
			mask |= (1 << atype.Shift) - 1
			min := maskl
			max := maskl | (^mask)
			step := 1 << atype.Shift
			errCheck += fmt.Sprintf("if %s & 0x%x != 0x%x {\n", arg, uint32(mask), uint32(maskl))
			errCheck += fmt.Sprintf("c.ctxt.Diag(\"Constant(%%d) must within the range of [%d,%d] in steps of %d\\n%%v\",%s,p)\n", min, max, step, arg)
			errCheck += fmt.Sprintf("}\n")
		}
		j++
	}
	buf.WriteString(errCheck)
	if insn.Words > 1 {
		fmt.Fprintf(buf, "out[1] = o1\n")
	}
	fmt.Fprintf(buf, "out[0] = o0\n")
	fmt.Fprintf(buf, "}\n")
	return buf.String()
}

// Generate a stringed name representing the type of arguments ISA
// instruction needs to be encoded into a usable machine instruction
func insnTypeStr(insn *Inst, uniqueRegTypes bool) string {
	if len(insn.Fields) == 0 {
		return "type_none"
	}

	ret := "type_"

	// Tag store opcodes to give special treatment when generating
	// assembler function. They encode similarly to their load analogues.
	if insn.memOp {
		if insn.memOpSt {
			ret += "st_"
		} else {
			ret += "ld_"
		}
	}

	// TODO: this is only sufficient for ISA3.1.
	for _, atype := range insn.Fields {
		switch atype.Type {
		// Simple, register like 5 bit field (CR bit, FPR, GPR, VR)
		case asm.TypeReg, asm.TypeFPReg, asm.TypeVecReg, asm.TypeCondRegBit:
			if uniqueRegTypes {
				ret += map[asm.ArgType]string{asm.TypeReg: "R", asm.TypeFPReg: "F", asm.TypeVecReg: "V", asm.TypeCondRegBit: "C"}[atype.Type]
				// Handle even/odd pairs in FPR/GPR args. They encode as 5 bits too, but odd values are invalid.
				if atype.Name[len(atype.Name)-1] == 'p' {
					ret += "p"
				}
			} else {
				ret += "R"
			}
		case asm.TypeMMAReg, asm.TypeCondRegField: // 3 bit register fields (MMA or CR field)
			ret += "M"
		case asm.TypeSpReg:
			ret += "P"
		case asm.TypeVecSReg: // VSX register (6 bits, usually split into 2 fields)
			ret += "X"
		case asm.TypeVecSpReg: // VSX register pair (5 bits, maybe split fields)
			ret += "Y"
		case asm.TypeImmSigned, asm.TypeOffset, asm.TypeImmUnsigned:
			if atype.Type == asm.TypeImmUnsigned {
				ret += "I"
			} else {
				ret += "S"
			}
			if atype.Shift != 0 {
				ret += fmt.Sprintf("%d", atype.Shift)
			}
		case asm.TypeNegOffset: // e.g offset in hashst rb, offset(ra)
			ret += "N"
		default:
			log.Fatalf("Unhandled type in insnTypeStr: %v\n", atype)
		}

		// And add bit packing info
		for _, bf := range atype.BitFields {
			ret += fmt.Sprintf("_%d_%d", bf.Word*32+bf.Offs, bf.Bits)
		}
	}
	return ret
}

type AggInfo struct {
	Insns []*Inst // List of instructions sharing this type
	Typef string  // The generated function name matching this
}

// Generate an Optab entry for a set of instructions with identical argument types
// and write it to buf.
func genOptabEntry(ta *AggInfo, typeMap map[string]*Inst) string {
	buf := new(bytes.Buffer)
	fitArg := func(f *Field, i *Inst) string {
		argToRegType := map[asm.ArgType]string{
			// TODO: only complete for ISA 3.1
			asm.TypeReg:          "C_REG",
			asm.TypeCondRegField: "C_CREG",
			asm.TypeCondRegBit:   "C_CRBIT",
			asm.TypeFPReg:        "C_FREG",
			asm.TypeVecReg:       "C_VREG",
			asm.TypeVecSReg:      "C_VSREG",
			asm.TypeVecSpReg:     "C_VSREG",
			asm.TypeMMAReg:       "C_AREG",
			asm.TypeSpReg:        "C_SPR",
		}
		if t, fnd := argToRegType[f.Type]; fnd {
			if f.Name[len(f.Name)-1] == 'p' {
				return t + "P"
			}
			return t
		}
		bits := f.Shift
		for _, sf := range f.BitFields {
			bits += sf.Bits
		}
		shift := ""
		if f.Shift != 0 {
			shift = fmt.Sprintf("S%d", f.Shift)
		}
		sign := "U"
		if f.Type == asm.TypeImmSigned || f.Type == asm.TypeOffset {
			sign = "S"
			// DS/DQ offsets should explicitly test their offsets to ensure
			// they are aligned correctly. This makes tracking down bad offset
			// passed to the compiler more straightfoward.
			if f.Type == asm.TypeOffset {
				shift = ""
			}
		}
		if f.Type == asm.TypeNegOffset {
			// This is a hack, but allows hashchk and like to correctly
			// merge there argument into a C_SOREG memory location type
			// argument a little later.
			sign = "S"
			bits = 16
			shift = ""
		}
		return fmt.Sprintf("C_%s%d%sCON", sign, bits, shift)
	}
	insn := ta.Insns[0]
	args := [6]string{}
	// Note, a2 is skipped if the second input argument does not map to a reg.
	argOrder := []int{
		5,
		0,
		1,
		2,
		3,
		4}

	i := 0
	for _, j := range insn.Fields {
		// skip a2 if it isn't a reg type.
		at := fitArg(&j, insn)
		if argOrder[i] == 1 && !strings.HasSuffix(at, "REG") {
			i++
		}
		args[argOrder[i]] = at
		i++
	}

	// Likewise, fixup memory operations. Combine imm + reg, reg + reg
	// operations into memory type arguments.
	if insn.memOp {
		switch args[0] + " " + args[1] {
		case "C_REG C_REG":
			args[0] = "C_XOREG"
		case "C_S16CON C_REG":
			args[0] = "C_SOREG"
		case "C_S34CON C_REG":
			args[0] = "C_LOREG"
		}
		args[1] = ""
		// Finally, fixup store operand ordering to match golang
		if insn.memOpSt {
			args[0], args[5] = args[5], args[0]
		}

	}
	fmt.Fprintf(buf, "{as: A%s,", opName(insn.Op))
	for i, s := range args {
		if len(s) <= 0 {
			continue
		}
		fmt.Fprintf(buf, "a%d: %s, ", i+1, s)
	}
	typef := typeMap[ta.Typef].Op

	pfx := ""
	if insn.Words > 1 {
		pfx = " ispfx: true,"
	}
	fmt.Fprintf(buf, "asmout: type_%s,%s size: %d},\n", typef, pfx, insn.Words*4)
	return buf.String()
}

// printEncoder implements the -fmt=encoder mode. This generates a go file named
// asm9_gtables.go.new. It is self-contained and is called into by the PPC64
// assembler routines.
//
// For now it is restricted to generating code for ISA 3.1 and newer, but it could
// support older ISA versions with some work, and integration effort.
func printEncoder(p *Prog) {
	const minISA = ISA_V31

	// The type map separates based on obj.Addr to a bit field.  Register types
	// for GPR, FPR, VR pack identically, but are classified differently.
	typeMap := map[string]*Inst{}
	typeAggMap := map[string]*AggInfo{}
	var oplistBuf bytes.Buffer
	var opnameBuf bytes.Buffer

	// The first opcode of 32 or 64 bits to appear in the opcode tables.
	firstInsn := [2]string{}

	// Sort the instructions by word size, then by ISA version, oldest to newest.
	sort.Slice(p.Insts, func(i, j int) bool {
		if p.Insts[i].Words != p.Insts[j].Words {
			return p.Insts[i].Words < p.Insts[j].Words
		}
		return p.Insts[i].order > p.Insts[j].order
	})

	// Classify each opcode and it's arguments, and generate opcode name/enum values.
	for i, insn := range p.Insts {
		if insn.Isa < minISA {
			continue
		}
		extra := ""
		if firstInsn[insn.Words-1] == "" {
			firstInsn[insn.Words-1] = opName(insn.Op)
			if insn.Words == 1 {
				extra = " = ALASTAOUT + iota"
			}
		}
		opType := insnTypeStr(&insn, false)
		opTypeOptab := insnTypeStr(&insn, true)
		fmt.Fprintf(&oplistBuf, "A%s%s\n", opName(insn.Op), extra)
		fmt.Fprintf(&opnameBuf, "\"%s\",\n", opName(insn.Op))
		// Use the oldest instruction to name the encoder function.  Some names
		// may change if minISA is lowered.
		if _, fnd := typeMap[opType]; !fnd {
			typeMap[opType] = &p.Insts[i]
		}
		at, fnd := typeAggMap[opTypeOptab]
		if !fnd {
			typeAggMap[opTypeOptab] = &AggInfo{[]*Inst{&p.Insts[i]}, opType}
		} else {
			at.Insns = append(at.Insns, &p.Insts[i])
		}
	}
	fmt.Fprintf(&oplistBuf, "ALASTGEN\n")
	fmt.Fprintf(&oplistBuf, "AFIRSTGEN = A%s\n", firstInsn[0])

	// Sort type information before outputing to ensure stable ordering
	targ := struct {
		InputFile   string
		Insts       []Inst
		MinISA      isaversion
		TypeAggList []*AggInfo
		TypeList    []*Inst
		FirstInsn   [2]string
		TypeMap     map[string]*Inst
		Oplist      string
		Opnames     string
	}{InputFile: inputFile, Insts: p.Insts, MinISA: minISA, FirstInsn: firstInsn, TypeMap: typeMap, Oplist: oplistBuf.String(), Opnames: opnameBuf.String()}
	for _, v := range typeAggMap {
		targ.TypeAggList = append(targ.TypeAggList, v)
	}
	for _, v := range typeMap {
		targ.TypeList = append(targ.TypeList, v)
	}
	sort.Slice(targ.TypeAggList, func(i, j int) bool {
		// Sort based on the first entry, it is the last to appear in Appendix F.
		return targ.TypeAggList[i].Insns[0].Op < targ.TypeAggList[j].Insns[0].Op
	})
	sort.Slice(targ.TypeList, func(i, j int) bool {
		return targ.TypeList[i].Op < targ.TypeList[j].Op
	})

	// Generate asm9_gtable.go from the following template.
	asm9_gtable_go := `
		// DO NOT EDIT
		// generated by: ppc64map -fmt=encoder {{.InputFile}}

		package ppc64

		import (
			"cmd/internal/obj"
		)

		const (
			{{print $.Oplist -}}
		)

		var GenAnames = []string {
			{{print $.Opnames -}}
		}

		var GenOpcodes = [...]uint32 {
			{{range $v := .Insts}}{{if ge $v.Isa $.MinISA -}}
			{{if (eq $v.Words 1)}}{{printf "0x%08x, // A%s" $v.Value  (opname $v.Op)}}
			{{else}}              {{printf "0x%08x, // A%s" $v.SValue (opname $v.Op)}}
			{{end}}{{end}}{{end -}}
		}

		var GenPfxOpcodes = [...]uint32 {
			{{range $v := .Insts}}{{if and (ge $v.Isa $.MinISA) (eq $v.Words 2) -}}
			{{printf "0x%08x, // A%s" $v.Value (opname $v.Op)}}
			{{end}}{{end -}}
		}

		var optabGen = []Optab {
			{{range $v := .TypeAggList -}}
			{{genoptabentry $v $.TypeMap -}}
			{{end -}}
		}

		{{range $v := .TypeList}}
		{{genencoderfunc $v $.FirstInsn}}
		{{end}}

		func opsetGen(from obj.As) bool {
			r0 := from & obj.AMask
			switch from {
			{{range $v := .TypeAggList -}}
			case A{{opname (index $v.Insns 0).Op}}:
				{{range $w := (slice $v.Insns 1) -}}
				opset(A{{opname $w.Op}},r0)
				{{end -}}
			{{end -}}
			default:
				return false
			}
			return true
		}
	`
	tmpl := template.New("asm9_gtable.go")
	tmpl.Funcs(template.FuncMap{
		"opname":         opName,
		"genencoderfunc": insnEncFuncStr,
		"genoptabentry":  genOptabEntry,
	})
	tmpl.Parse(asm9_gtable_go)

	// Write and gofmt the new file.
	var tbuf bytes.Buffer
	if err := tmpl.Execute(&tbuf, targ); err != nil {
		log.Fatal(err)
	}
	tout, err := gofmt.Source(tbuf.Bytes())
	if err != nil {
		fmt.Printf("%s", tbuf.Bytes())
		log.Fatalf("gofmt error: %v", err)
	}
	if err := os.WriteFile("asm9_gtables.go.new", tout, 0666); err != nil {
		log.Fatalf("Failed to create asm9_gtables.new: %v", err)
	}
}

// printASM implements the -fmt=asm mode.  This prints out a gnu assembler file
// which can be used to used to generate test output to verify the golang
// disassembler's gnu output matches gnu binutils. This is used as an input to
// ppc64util to generate the decode_generated.txt test case.
func printASM(p *Prog) {
	fmt.Printf("#include \"hack.h\"\n")
	fmt.Printf(".text\n")
	for _, inst := range p.Insts {
		// Prefixed load/stores have extra restrictions with D(RA) and R. Rename them
		// To simplify generation.
		str := inst.Encoding
		if str[0] == 'p' && str[len(str)-1] == 'R' {
			str = strings.Replace(str, "D(RA),R", "Dpfx(RApfx),Rpfx", 1)
			str = strings.Replace(str, "RA,SI,R", "RApfx,SIpfx,Rpfx", 1)
		}
		fmt.Printf("\t%s\n", str)
	}
}

// opName translate an opcode to a valid Go identifier all-cap op name.
func opName(op string) string {
	return strings.ToUpper(strings.Replace(op, ".", "CC", 1))
}

// argFieldName constructs a name for the argField
func argFieldName(f Field) string {
	ns := []string{"ap", f.Type.String()}
	for _, b := range f.BitFields {
		ns = append(ns, fmt.Sprintf("%d_%d", b.Word*32+b.Offs, b.Word*32+b.Offs+b.Bits-1))
	}
	if f.Shift > 0 {
		ns = append(ns, fmt.Sprintf("shift%d", f.Shift))
	}
	return strings.Join(ns, "_")
}

var funcBodyTmpl = template.Must(template.New("funcBody").Parse(``))

// printDecoder implements the -fmt=decoder mode.
// It emits the tables.go for package armasm's decoder.
func printDecoder(p *Prog) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "// Code generated by ppc64map -fmt=decoder %s DO NOT EDIT.\n", inputFile)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "package ppc64asm\n\n")

	// Build list of opcodes, using the csv order (which corresponds to ISA docs order)
	m := map[string]bool{}
	fmt.Fprintf(&buf, "const (\n\t_ Op = iota\n")
	for _, inst := range p.Insts {
		name := opName(inst.Op)
		if ok := m[name]; ok {
			continue
		}
		m[name] = true
		fmt.Fprintf(&buf, "\t%s\n", name)
	}
	fmt.Fprint(&buf, ")\n\n\n")

	// Emit slice mapping opcode number to name string.
	m = map[string]bool{}
	fmt.Fprintf(&buf, "var opstr = [...]string{\n")
	for _, inst := range p.Insts {
		name := opName(inst.Op)
		if ok := m[name]; ok {
			continue
		}
		m[name] = true
		fmt.Fprintf(&buf, "\t%s: %q,\n", opName(inst.Op), inst.Op)
	}
	fmt.Fprint(&buf, "}\n\n\n")

	// print out argFields
	fmt.Fprintf(&buf, "var (\n")
	m = map[string]bool{}
	for _, inst := range p.Insts {
		for _, f := range inst.Fields {
			name := argFieldName(f)
			if ok := m[name]; ok {
				continue
			}
			m[name] = true
			fmt.Fprintf(&buf, "\t%s = &argField{Type: %#v, Shift: %d, BitFields: BitFields{", name, f.Type, f.Shift)
			for _, b := range f.BitFields {
				fmt.Fprintf(&buf, "{%d, %d, %d},", b.Offs, b.Bits, b.Word)
			}
			fmt.Fprintf(&buf, "}}\n")
		}
	}
	fmt.Fprint(&buf, ")\n\n\n")

	// Emit decoding table.
	fmt.Fprintf(&buf, "var instFormats = [...]instFormat{\n")
	for _, inst := range p.Insts {
		m, v, dc := uint64(inst.Mask)<<32, uint64(inst.Value)<<32, uint64(inst.DontCare)<<32
		m, v, dc = uint64(inst.SMask)|m, uint64(inst.SValue)|v, uint64(inst.SDontCare)|dc
		fmt.Fprintf(&buf, "\t{ %s, %#x, %#x, %#x,", opName(inst.Op), m, v, dc)
		fmt.Fprintf(&buf, " // %s (%s)\n\t\t[6]*argField{", inst.Text, inst.Encoding)
		for _, f := range inst.Fields {
			fmt.Fprintf(&buf, "%s, ", argFieldName(f))
		}
		fmt.Fprintf(&buf, "}},\n")
	}
	fmt.Fprint(&buf, "}\n\n")

	out, err := gofmt.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("gofmt error: %v", err)
		fmt.Printf("%s", buf.Bytes())
	} else {
		fmt.Printf("%s", out)
	}
}
