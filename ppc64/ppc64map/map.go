// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ppc64map constructs the ppc64 opcode map from the instruction set CSV file.
//
// Usage:
//	ppc64map [-fmt=format] ppc64.csv
//
// The known output formats are:
//
//  text (default) - print decoding tree in text form
//  decoder - print decoding tables for the ppc64asm package
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	gofmt "go/format"
	"log"
	"math/bits"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	asm "golang.org/x/arch/ppc64/ppc64asm"
)

var format = flag.String("fmt", "text", "output format: text, decoder, asm")
var debug = flag.Bool("debug", false, "enable debugging output")

var inputFile string

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
	Insts    []Inst
	OpRanges map[string]string
}

type Field struct {
	Name      string
	BitFields asm.BitFields
	Type      asm.ArgType
	Shift     uint8
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
func add(p *Prog, text, mnemonics, encoding, tags string) {
	// Parse encoding, building size and offset of each field.
	// The first field in the encoding is the smallest offset.
	// And note the MSB is bit 0, not bit 31.
	// Example: "31@0|RS@6|RA@11|///@16|26@21|Rc@31|"
	var args, pargs Args
	var pmask, pvalue, presv, resv uint32
	iword := int8(0)
	ispfx := false

	// Is this a prefixed instruction?
	if encoding[0] == ',' {
		pfields := strings.Split(encoding, ",")[1:]

		if len(pfields) != 2 {
			fmt.Fprintf(os.Stderr, "%s: Prefixed instruction must be 2 words long.\n", text)
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

			case "XMSK", "YMSK", "PMSK", "IX":
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

			case "SPR", "DCRN", "BHRBE", "TBR", "SR", "TMR", "PMRN": // Note: if you add to this list and the register field needs special handling, add it to switch statement below
				typ = asm.TypeSpReg
				switch opr {
				case "DCRN":
					opr = "DCR"
				}
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
			if f2.Bits > 0 {
				field.BitFields.Append(f2)
			}
			if f3.Bits > 0 {
				field.BitFields.Append(f3)
			}
			inst.Fields = append(inst.Fields, field)
		}
		if *debug {
			fmt.Printf("%v\n", inst)
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

	fmt.Fprintf(&buf, "// DO NOT EDIT\n")
	fmt.Fprintf(&buf, "// generated by: ppc64map -fmt=decoder %s\n", inputFile)
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
