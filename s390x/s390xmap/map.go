// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// s390xmap constructs the s390x opcode map from the instruction set CSV file.
//
// Usage:
//
//	s390map [-fmt=format] s390x.csv
//
// The known output formats are:
//
//	text (default) - print decoding tree in text form
//	decoder - print decoding tables for the s390xasm package
//	encoder - generate a self-contained file which can be used to encode
//		  go obj.Progs into machine code
//	asm - generate a GNU asm file which can be compiled by gcc containing
//	      all opcodes discovered in s390x.csv using macro friendly arguments.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	gofmt "go/format"
	asm "golang.org/x/arch/s390x/s390xasm"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var format = flag.String("fmt", "text", "output format: text, decoder, asm")
var debug = flag.Bool("debug", false, "enable debugging output")

var inputFile string

func usage() {
	fmt.Fprintf(os.Stderr, "usage: s390xmap [-fmt=format] s390x.csv\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("s390xmap: ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}

	inputFile = flag.Arg(0)

	var printTyp func(*Prog)
	switch *format {
	default:
		log.Fatalf("unknown output format %q", *format)
	case "text":
		printTyp = printText
	case "decoder":
		printTyp = printDecoder
	case "asm":
		printTyp = printASM
	case "encoder":
		printTyp = printEncoder
	}

	p, err := readCSV(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Parsed %d instruction forms.", len(p.Insts))
	printTyp(p)
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
	if len(table[0]) < 3 {
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
	Name     string
	BitField asm.BitField
	Type     asm.ArgType
	flags    uint16
}

func (f Field) String() string {
	return fmt.Sprintf("%v(%s%v)", f.Type, f.Name, f.BitField)
}

type Inst struct {
	Text     string
	Encoding string
	Op       string
	Mask     uint64
	Value    uint64
	DontCare uint64
	Len      uint16
	Fields   []Field
}

func (i Inst) String() string {
	return fmt.Sprintf("%s (%s) %08x/%08x %v (%s)", i.Op, i.Encoding, i.Value, i.Mask, i.Fields, i.Text)
}

type Arg struct {
	Name string
	Bits int8
	Offs int8
}

func (a Arg) String() string {
	return fmt.Sprintf("%s[%d:%d]", a.Name, a.Offs, a.Offs+a.Bits-1)
}

func (a Arg) Maximum() int {
	return 1<<uint8(a.Bits) - 1
}

func (a Arg) BitMask() uint64 {
	return uint64(a.Maximum()) << a.Shift()
}

func (a Arg) Shift() uint8 {
	return uint8(64 - a.Offs - a.Bits)
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

// Split the string encoding into an Args. The encoding string loosely matches the regex
// (arg@bitpos|)+
func parseFields(encoding, text string) Args {
	var err error
	var args Args

	fields := strings.Split(encoding, "|")

	for i, f := range fields {
		name, off := "", -1
		if f == "" {
			off = 64
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
			off, err = strconv.Atoi(f[j+1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "err for: %s has: %s for %s\n", f[:j], err, f[j+1:])
			}
			name = f[:j]
		}
		if len(args) > 0 {
			args[len(args)-1].Bits += int8(off)
		}
		if name != "" && name != "??" {
			arg := Arg{Name: name, Offs: int8(off), Bits: int8(-off)}
			args.Append(arg)
		}
	}
	return args
}

// Compute the Mask (usually Opcode + secondary Opcode bitfields),
// the Value (the expected value under the mask), and
// reserved bits (i.e the // fields which should be set to 0)
func computeMaskValueReserved(args Args, text string) (mask, value, reserved uint64) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		v, err := strconv.Atoi(arg.Name)
		switch {
		case err == nil && v >= 0: // is a numbered field
			if v < 0 || v > arg.Maximum() {
				fmt.Fprintf(os.Stderr, "%s: field %s value (%d) is out of range (%d-bit)\n", text, arg, v, arg.Bits)
			}
			mask |= arg.BitMask()
			value |= uint64(v) << arg.Shift()
			args.Delete(i)
			i--
		case arg.Name[0] == '/': // don't care
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
	// sanity checks
	if mask&reserved != 0 {
		log.Fatalf("%s: mask (%08x) and don't care (%08x) collide", text, mask, reserved)
	}
	if value&^mask != 0 {
		log.Fatalf("%s: value (%08x) out of range of mask (%08x)", text, value, mask)
	}
	return
}

func Imm_signed_8bit_check(op string) bool {
	imm_8 := []string{"ASI", "AGSI", "ALSI", "ALGSI", "CIB", "CGIB", "CIJ", "CGIJ", "NI", "NIY", "OI", "OIY", "XI", "XIY"}
	var ret bool
	ret = false
	for _, str := range imm_8 {
		if strings.Compare(op, str) == 0 {
			ret = true
			break
		}
	}
	return ret
}

func Imm_signed_16bit_check(op string) bool {
	imm_16 := []string{"AHI", "AGHI", "ALHSIK", "ALGHSIK", "AHIK", "AGHIK", "LHI", "LGHI", "MVGHI", "CIT", "CGIT", "CGHI", "CGHSI", "CHHSI", "CHI", "CHSI", "CRJ", "CGRJ", "NIHH", "NILL", "NIHL", "NILH", "LLIHH", "LLILL", "LLIHL", "LLILH", "OIHH", "OILL", "OIHL", "OILH", "VLEIB", "VLEIH", "VLEIF", "VLEIG"}
	var ret bool
	ret = false
	for _, str := range imm_16 {
		if strings.Compare(op, str) == 0 {
			ret = true
			break
		}
	}
	return ret
}

func Imm_signed_32bit_check(op string) bool {
	imm_32 := []string{"AFI", "AGFI", "AIH", "CIH", "CFI", "CGFI", "CRL", "STRL", "STGRL", "LGFI", "LLIHF", "LLILF", "MSFI", "MSGFI", "MGHI", "MHI", "NIHF", "NILF", "OILF", "OIHF", "XILF", "XIHF"}
	var ret bool
	ret = false
	for _, str := range imm_32 {
		if strings.Compare(op, str) == 0 {
			ret = true
			break
		}
	}
	return ret
}

func check_flags(flags string) bool {
	if strings.Contains(flags, "Da") {
		return true
	} else if strings.Contains(flags, "Db") {
		return true
	} else if strings.Contains(flags, "Dt") {
		return true
	} else {
		return false
	}
}

// Parse a row from the CSV describing the instructions, and place the
// detected instructions into p. One entry may generate multiple intruction
// entries as each extended mnemonic listed in text is treated like a unique
// instruction.
func add(p *Prog, text, mnemonics, encoding, flags string) {
	// Parse encoding, building size and offset of each field.
	// The first field in the encoding is the smallest offset.
	// And note the MSB is bit 0, not bit 31.
	// Example: "31@0|RS@6|RA@11|///@16|26@21|Rc@31|"
	var args Args

	args = parseFields(encoding, text)
	mask, value, dontCare := computeMaskValueReserved(args, text)

	// split mnemonics into individual instructions
	inst := Inst{Text: text, Encoding: mnemonics, Value: value, Mask: mask, DontCare: dontCare}

	// order inst.Args according to mnemonics order
	for i, opr := range operandRe.FindAllString(mnemonics, -1) {
		if i == 0 { // operation
			inst.Op = opr
			continue
		}
		field := Field{Name: opr}
		typ := asm.TypeUnknown
		flag := uint16(0)
		switch opr {
		case "R1", "R2", "R3":
			s := strings.Split(mnemonics, " ")
			switch opr {
			case "R1":
				switch s[0] {
				case "CPDT", "CPXT", "CDXT", "CZXT", "CZDT":
					typ = asm.TypeFPReg
					flag = 0x2
				case "CUXTR", "EEXTR", "EEDTR", "EFPC", "ESXTR", "ESDTR", "LGDR", "SFPC", "SFASR":
					typ = asm.TypeReg
					flag = 0x1
				case "CPYA", "LAM", "LAMY", "STAM", "STAMY", "SAR", "TAR":
					typ = asm.TypeACReg
					flag = 0x3
				case "LCTL", "LCTLG", "STCTL", "STCTG":
					typ = asm.TypeCReg
					flag = 0x4
				default:
					if check_flags(flags) {
						if strings.Contains(text, "CONVERT TO") {
							typ = asm.TypeReg
							flag = 0x1
						} else {
							typ = asm.TypeFPReg
							flag = 0x2
						}
					} else {
						typ = asm.TypeReg
						flag = 0x1
					}
				}
			case "R2":
				switch s[0] {
				case "IEXTR", "IEDTR", "LDGR", "RRXTR", "RRDTR":
					typ = asm.TypeReg
					flag = 0x1
				case "CPYA", "EAR":
					typ = asm.TypeACReg
					flag = 0x3
				default:
					if check_flags(flags) {
						if strings.Contains(text, "CONVERT FROM") {
							typ = asm.TypeReg
							flag = 0x1
						} else {
							typ = asm.TypeFPReg
							flag = 0x2
						}
					} else {
						typ = asm.TypeReg
						flag = 0x1
					}
				}
			case "R3":
				switch s[0] {
				case "LAM", "LAMY", "STAM", "STAMY":
					typ = asm.TypeACReg
					flag = 0x3
				case "LCTL", "LCTLG", "STCTL", "STCTG":
					typ = asm.TypeCReg
					flag = 0x4
				default:
					if check_flags(flags) {
						typ = asm.TypeFPReg
						flag = 0x2
					} else {
						typ = asm.TypeReg
						flag = 0x1
					}
				}
			}

		case "I", "I1", "I2", "I3", "I4", "I5":
			flag = 0x0
			switch opr {
			case "I", "I1":
				typ = asm.TypeImmUnsigned

			case "I2":
				if Imm_signed_8bit_check(inst.Op) {
					typ = asm.TypeImmSigned8
					break
				} else if Imm_signed_16bit_check(inst.Op) { // "ASI", "AGSI", "ALSI", "ALGSI"
					typ = asm.TypeImmSigned16
					break
				} else if Imm_signed_32bit_check(inst.Op) { // "AHI", "AGHI", "AHIK", "AGHIK", "LHI", "LGHI"
					typ = asm.TypeImmSigned32
					break
				} else {
					typ = asm.TypeImmUnsigned
					break
				}

			case "I3", "I4", "I5":
				typ = asm.TypeImmUnsigned

			}

		case "RI2", "RI3", "RI4":
			flag = 0x80
			i := args.Find(opr)
			count := uint8(args[i].Bits)
			if count == 12 {
				typ = asm.TypeRegImSigned12
				break
			} else if count == 16 {
				typ = asm.TypeRegImSigned16
				break
			} else if count == 24 {
				typ = asm.TypeRegImSigned24
				break
			} else if count == 32 {
				typ = asm.TypeRegImSigned32
				break
			}

		case "M1", "M3", "M4", "M5", "M6":
			flag = 0x800
			typ = asm.TypeMask

		case "B1", "B2", "B3", "B4":
			typ = asm.TypeBaseReg
			flag = 0x20 | 0x01

		case "X2":
			typ = asm.TypeIndexReg
			flag = 0x40 | 0x01

		case "D1", "D2", "D3", "D4":
			flag = 0x10
			i := args.Find(opr)
			if uint8(args[i].Bits) == 20 {
				typ = asm.TypeDispSigned20
				break
			} else {
				typ = asm.TypeDispUnsigned
				break
			}

		case "L1", "L2":
			typ = asm.TypeLen
			flag = 0x10
		case "V1", "V2", "V3", "V4", "V5", "V6":
			typ = asm.TypeVecReg
			flag = 0x08
		}

		if typ == asm.TypeUnknown {
			log.Fatalf("%s %s unknown type for opr %s", text, inst, opr)
		}
		field.Type = typ
		field.flags = flag
		var f1 asm.BitField
		i := args.Find(opr)
		if i < 0 {
			log.Fatalf("%s: couldn't find %s in %s", text, opr, args)
		}
		f1.Offs, f1.Bits = uint8(args[i].Offs), uint8(args[i].Bits)
		field.BitField = f1
		inst.Fields = append(inst.Fields, field)
	}
	if strings.HasPrefix(inst.Op, "V") || strings.Contains(inst.Op, "WFC") || strings.Contains(inst.Op, "WFK") { //Check Vector Instructions
		Bits := asm.BitField{Offs: 36, Bits: 4}
		field := Field{Name: "RXB", BitField: Bits, Type: asm.TypeImmUnsigned, flags: 0xC00}
		inst.Fields = append(inst.Fields, field)
	}
	if *debug {
		fmt.Printf("%v\n", inst)
	}
	p.Insts = append(p.Insts, inst)
}

// operandRe matches each operand (including opcode) in instruction mnemonics
var operandRe = regexp.MustCompile(`([[:alpha:]][[:alnum:]_]*\.?)`)

// printText implements the -fmt=text mode, which is not implemented (yet?).
func printText(p *Prog) {
	log.Fatal("-fmt=text not implemented")
}

// printEncoder implements the -fmt=encoder mode. which is not implemented (yet?).
func printEncoder(p *Prog) {
	log.Fatal("-fmt=encoder not implemented")
}

func printASM(p *Prog) {
	fmt.Printf("#include \"hack.h\"\n")
	fmt.Printf(".text\n")
	for _, inst := range p.Insts {
		fmt.Printf("\t%s\n", inst.Encoding)
	}
}

// argFieldName constructs a name for the argField
func argFieldName(f Field) string {
	ns := []string{"ap", f.Type.String()}
	b := f.BitField
	ns = append(ns, fmt.Sprintf("%d_%d", b.Offs, b.Offs+b.Bits-1))
	return strings.Join(ns, "_")
}

// printDecoder implements the -fmt=decoder mode.
// It emits the tables.go for package armasm's decoder.
func printDecoder(p *Prog) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "// Code generated by s390xmap -fmt=decoder %s DO NOT EDIT.\n", inputFile)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "package s390xasm\n\n")

	// Build list of opcodes, using the csv order (which corresponds to ISA docs order)
	m := map[string]bool{}
	fmt.Fprintf(&buf, "const (\n\t_ Op = iota\n")
	for i := 0; i < len(p.Insts); i++ {
		name := p.Insts[i].Op
		switch name {
		case "CUUTF", "CUTFU", "PPNO":
			m[name] = false
			p.Insts = append(p.Insts[:i], p.Insts[i+1:]...)
			i--
		default:
			m[name] = true
		}
		if ok := m[name]; !ok {
			continue
		}
		fmt.Fprintf(&buf, "\t%s\n", name)
	}
	fmt.Fprint(&buf, ")\n\n\n")

	// Emit slice mapping opcode number to name string.
	m = map[string]bool{}
	fmt.Fprintf(&buf, "var opstr = [...]string{\n")
	for _, inst := range p.Insts {
		name := inst.Op
		if ok := m[name]; ok {
			continue
		}
		m[name] = true
		fmt.Fprintf(&buf, "\t%s: %q,\n", inst.Op, strings.ToLower(inst.Op))
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
			fmt.Fprintf(&buf, "\t%s = &argField{Type: %#v, flags: %#x, BitField: BitField", name, f.Type, f.flags)
			b := f.BitField
			fmt.Fprintf(&buf, "{%d, %d }", b.Offs, b.Bits)
			fmt.Fprintf(&buf, "}\n")
		}
	}
	fmt.Fprint(&buf, ")\n\n\n")

	// Emit decoding table.
	fmt.Fprintf(&buf, "var instFormats = [...]instFormat{\n")
	for _, inst := range p.Insts {
		m, v, dc := inst.Mask, inst.Value, inst.DontCare
		fmt.Fprintf(&buf, "\t{ %s, %#x, %#x, %#x,", inst.Op, m, v, dc)
		fmt.Fprintf(&buf, " // %s (%s)\n\t\t[8]*argField{", inst.Text, inst.Encoding)
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
