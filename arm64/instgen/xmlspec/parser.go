// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xmlspec implements the parser of the A64 instruction set XML specification.
// It parses the XML files and returns a list of Instruction objects.
// The expected data is fetched from:
//
//	https://developer.arm.com/-/cdn-downloads/permalink/Exploration-Tools-A64-ISA/ISA_A64/ISA_A64_xml_A_profile-2025-12.tar.gz
//
// Pass directory ISA_A64_xml_A_profile-2025-12 to ParseXMLFiles to get the instructions.
//
// Currently the parser only processes SVE and SVE2 instructions.
// Other instructions will still be unmarshalled but they won't have processing logics.
package xmlspec

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var debug = flag.Int("debug", 0, "enable debug output")

var (
	reZREG = regexp.MustCompile(`(^|[^/])<Z[A-Za-z1-9]+>`)
	rePREG = regexp.MustCompile(`<P[A-Za-z1-9]+>`)
)

type fixedElemType int

const (
	FixedArrangement fixedElemType = iota
	FixedLSL
	FixedSXTW
	FixedUXTW
	FixedModAmt
)

type fixedElemRule struct {
	re  *regexp.Regexp
	t   fixedElemType
	val string
}

var fixedElemRules = []fixedElemRule{
	{regexp.MustCompile(`\.B`), FixedArrangement, "B"},
	{regexp.MustCompile(`\.H`), FixedArrangement, "H"},
	{regexp.MustCompile(`\.S`), FixedArrangement, "S"},
	{regexp.MustCompile(`\.D`), FixedArrangement, "D"},
	{regexp.MustCompile(`\.Q`), FixedArrangement, "Q"},
	{regexp.MustCompile(`\.N`), FixedArrangement, "N"},
	{regexp.MustCompile(`LSL #1`), FixedLSL, "1"},
	{regexp.MustCompile(`LSL #2`), FixedLSL, "2"},
	{regexp.MustCompile(`LSL #3`), FixedLSL, "3"},
	{regexp.MustCompile(`LSL #4`), FixedLSL, "4"},
	{regexp.MustCompile(`SXTW`), FixedSXTW, "SXTW"},
	{regexp.MustCompile(`UXTW`), FixedUXTW, "UXTW"},
	// FixedModAmt rules are special, they requires a mapping of
	// the preceding elem's symbol to be <mod>.
	{regexp.MustCompile(`#1`), FixedModAmt, "1"},
	{regexp.MustCompile(`#2`), FixedModAmt, "2"},
	{regexp.MustCompile(`#3`), FixedModAmt, "3"},
	{regexp.MustCompile(`#4`), FixedModAmt, "4"},
	{regexp.MustCompile(`#8`), FixedModAmt, "8"},
}

type operandRule struct {
	re    *regexp.Regexp
	class string
}

var operandRules = []operandRule{
	// AC_REG: Standard scalar registers (W, X, R).
	{regexp.MustCompile(`^(<[WX][a-z]+>!?|<R><[a-z]+>|X[0-9]+|{<[WX][a-z]+>})$`), "AC_REG"},
	// AC_RSP: Scalar registers or stack pointer (SP).
	{regexp.MustCompile(`^<([WX][a-z]{1}|R><n)\|[W]?SP>$`), "AC_RSP"},
	// AC_PREG: Predicate registers (P).
	{regexp.MustCompile(`^<P[a-z]{1}>$`), "AC_PREG"},
	// AC_PREG: Predicate-as-counter registers (PN).
	{regexp.MustCompile(`^<PN[a-z]{1}>$`), "AC_PREG"},
	// AC_PREGZM: Predicate registers with merging predication (/M).
	{regexp.MustCompile(`^<P[N]?[a-z]{1}>\/M$`), "AC_PREGZM"},
	// AC_PREGZM: Predicate registers with zeroing predication (/Z).
	{regexp.MustCompile(`^<P[N]?[a-z]{1}>\/(Z|<ZM>)$`), "AC_PREGZM"},
	// AC_REGIDX: Registers with immediate index.
	{regexp.MustCompile(`^(<[PZ][N]?[a-z]{1}>|ZT0)\[<[a-z]+>\]$`), "AC_REGIDX"},
	// AC_ZREG: Scalable vector registers (Z).
	{regexp.MustCompile(`^<Z[a-z]+>$`), "AC_ZREG"},
	// AC_ARNG: Registers with arrangement (e.g. .B, .D, .S) or type variable (<T>).
	{regexp.MustCompile(`^<[PVZ][a-zA-Z]+>\.([1-9]*[BDHQS]|<T[a-z]*>)$`), "AC_ARNG"},
	// AC_ARNGIDX: Register arrangement with index.
	{regexp.MustCompile(`^<[VZ][a-zA-Z]*>\.([1-9]*[BDHQS]|<T[a-z]*>)\[(<(index|imm)[1-9]*>|[0-9]+)\]$`), "AC_ARNGIDX"},
	{regexp.MustCompile(`^{[\s]+<[PVZ][a-z]+[1-4]*>\.[BDHQS],*[\s]*}\[<index>\]$`), "AC_ARNGIDX"},
	// AC_REGLIST1: List of 1 register with arrangement.
	{regexp.MustCompile(`^{[\s]+<[PVZ][a-z]+>\.([1-9]*[BDHQS]|<T[a-z]*>)[\s]+}$`), "AC_REGLIST1"},
	// AC_REGLIST2: List of 2 registers with arrangement.
	{regexp.MustCompile(`^{[\s]+(<[PVZ][a-z]+([1-2]|\+[1-2])*>\.([1-9]*[BDHQS]|<T[a-z]*>),*[\s]*){2}}$`), "AC_REGLIST2"},
	// AC_REGLIST3: List of 3 registers with arrangement.
	{regexp.MustCompile(`^{[\s]+(<[PVZ][a-z]+([1-3]|\+[1-3])*>\.([1-9]*[BDHQS]|<T[a-z]*>)(,|-)*[\s]*){3}}$`), "AC_REGLIST3"},
	// AC_REGLIST4: List of 4 registers with arrangement.
	{regexp.MustCompile(`^{[\s]+(<[PVZ][a-z]+([1-4]|\+[1-4])*>\.([1-9]*[BDHQS]|<T[a-z]*>),*[\s]*){4}}$`), "AC_REGLIST4"},
	// AC_REGLIST_RANGE: List of registers in a range.
	{regexp.MustCompile(`^{[\s]+(<[PVZ][a-z]+[1-2]*>\.([1-9]*[BDHQS]|<T[a-z]*>)-*[\s]*){2}}$`), "AC_REGLIST_RANGE"},
	{regexp.MustCompile(`^{[\s]+(<[PVZ][a-z]+[14]>\.([BDHQS]|<T[a-z]*>)-*[\s]*){2}}$`), "AC_REGLIST_RANGE"}, // It's 4 registers, but in the mnemonic it's 2.
	// AC_MEMOFF: Memory operand with immediate offset.
	{regexp.MustCompile(`^\[<Xn\|SP>([\s]*\{,[\s]*#([0-9]+|<[a-z]+>)\})*\]$`), "AC_MEMOFF"},
	{regexp.MustCompile(`^\[<Z[a-z]+>\.[BDHQS](\{,[\s]*#<[a-z]+>\})*\]$`), "AC_MEMOFF"},
	// AC_MEMOFFMULVL: Memory operand with immediate offset that is multiplied by the vector's in-memory size.
	{regexp.MustCompile(`^\[<Xn\|SP>[\s]*\{,[\s]*#<[a-z]+>,[\s]*MUL[\s]+VL[\s]*\}\]$`), "AC_MEMOFFMULVL"},
	// AC_MEMEXT: Memory operand with register offset and optional extension (signed or unsigned) or shift (logical shift left).
	{regexp.MustCompile(`^\[<Xn\|SP>,[\s]*(<X[a-z]+>|<Z[a-z]+>\.[BDHQS])\]$`), "AC_MEMEXT"},
	{regexp.MustCompile(`^\[<Xn\|SP>\{?,[\s]*<X[a-z]+>\{?,[\s]*LSL[\s]+(<amount>|#[0-9]+)\}?\]$`), "AC_MEMEXT"},
	{regexp.MustCompile(`^\[(<Xn\|SP>|<Z[a-z]+>\.[BDHQS])\{,[\s]*<X[a-z]+>\}\]$`), "AC_MEMEXT"},
	{regexp.MustCompile(`^\[<Xn\|SP>,[\s]*<Z[a-z]+>\.[BDHQS],[\s]*(<mod>([\s]+#[0-9]+)*|LSL[\s]+#[0-9]+)\]$`), "AC_MEMEXT"},
	{regexp.MustCompile(`^\[<Z[a-z]+>\.(<T>|[BDHQS]),[\s]*<Z[a-z]+>\.(<T>|[BDHQS])\{?,[\s]*(<mod>|SXTW\{?|UXTW\{?)[\s]*<amount>\}?\]$`), "AC_MEMEXT"},
	// AC_SPECIAL: Prefetch operation.
	{regexp.MustCompile(`^<prfop>$`), "AC_SPECIAL"},
	// AC_SPECIAL: Vector length.
	{regexp.MustCompile(`^<vl>$`), "AC_SPECIAL"},
	// AC_REG_PATTERN: Register with rotate/replication pattern.
	{regexp.MustCompile(`^<[WX][dn]+>(\{\s*,\s*<pattern>(\{\s*,\s*MUL\s+#<imm>\s*\})?\s*\})?$`), "AC_REG_PATTERN"},
	// AC_ZREG_PATTERN: Z register with rotate/replication pattern.
	{regexp.MustCompile(`^<Z[dn]+>\.(<T>|[BDHQS])(\{\s*,\s*<pattern>(\{\s*,\s*MUL\s+#<imm>\s*\})?\s*\})?$`), "AC_ZREG_PATTERN"},
	// AC_PREGIDX: Predicate register with index.
	{regexp.MustCompile(`^<P[nm]>\.<T>\[\s*<Wv>\s*,\s*<imm>\s*\]$`), "AC_PREGIDX"},
	// AC_PREG_PATTERN: Predicate register with pattern.
	{regexp.MustCompile(`^<P[dn]>\.<T>(\{\s*,\s*<pattern>\s*\})?$`), "AC_PREG_PATTERN"},
	// AC_ZREGIDX: Z register with optional index.
	{regexp.MustCompile(`^<Z[dn]>(\{\s*\[<imm>\]\s*\})?$`), "AC_ZREGIDX"},
	// AC_IMM: Immediate value.
	{regexp.MustCompile(`(^#.*)|(<const>)$`), "AC_IMM"},
	// AC_VREG: V registers (SIMD).
	{regexp.MustCompile(`(^<Dd>|^<V>.*)$`), "AC_VREG"},
}

// warmUpCache initializes the XML decoding cache for the Instruction type.
// This is necessary because encoding/xml uses reflect to build a cache of
// struct fields, and this process is not thread-safe if multiple goroutines
// attempt to unmarshal into the same type for the first time concurrently.
func warmUpCache() {
	var inst InstructionParsed
	// Unmarshal a more complete XML to warm up the cache for nested types.
	// This ensures that reflection data for all referenced types is initialized
	// sequentially before parallel workers start.
	dummyXML := `
		<instructionsection>
			<docvars>
				<docvar key="a" value="b"/>
			</docvars>
			<classes>
				<iclass>
					<encoding name="e">
						<box hibit="31" width="1" name="n">
							<c>1</c>
						</box>
						<asmtemplate>
							<text>ADD</text>
							<a link="s" hover="h">X0</a>
						</asmtemplate>
					</encoding>
				</iclass>
			</classes>
			<explanations>
				<explanation>
					<symbol link="s">X0</symbol>
					<account encodedin="e">
						<intro>
							<para>text</para>
						</intro>
					</account>
					<definition encodedin="e">
						<intro>text</intro>
						<table>
							<tgroup cols="1">
								<thead>
									<row>
										<entry>Val</entry>
									</row>
								</thead>
								<tbody>
									<row>
										<entry>1</entry>
									</row>
								</tbody>
							</tgroup>
						</table>
					</definition>
				</explanation>
			</explanations>
		</instructionsection>
	`
	_ = xml.Unmarshal([]byte(dummyXML), &inst)
}

func init() {
	warmUpCache()
}

func ParseXMLFiles(dir string) []*InstructionParsed {
	log.Println("Start parsing the xml files")
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	insts := make([]*InstructionParsed, len(files))

	for i, file := range files {
		fileName := file.Name()
		if ext := path.Ext(fileName); ext != ".xml" {
			continue
		}
		wg.Add(1)
		fileName = path.Join(dir, fileName)
		go func(name string, i int) {
			defer wg.Done()
			if inst := parse(name); inst != nil {
				insts[i] = inst
			}
		}(fileName, i)
	}
	wg.Wait()

	log.Println("Finish parsing the xml files")
	return insts
}

// parse parses an xml file and returns the instruction.
func parse(f string) *InstructionParsed {
	xmlFile, err := os.Open(f)
	if err != nil {
		log.Fatalf("Open file %s failed: %v\n", f, err)
	}
	defer xmlFile.Close()
	byteValue, err := io.ReadAll(xmlFile)
	if err != nil {
		log.Fatalf("io.ReadAll %s failed: %v\n", f, err)
	}

	var inst = new(InstructionParsed)
	inst.file = f
	if err = xml.Unmarshal(byteValue, inst); err != nil {
		// Ignore non-instruction files.
		if strings.HasPrefix(err.Error(), "expected element type <instructionsection>") {
			return nil
		}
		log.Fatalf("Unmarshal %s failed: %v\n", f, err)
	}
	if inst.Type != "instruction" && inst.Type != "alias" {
		return nil
	}

	return inst
}

func (inst *InstructionParsed) setBinary(code, bitVal uint32, value string) uint32 {
	switch value {
	case "0", "(0)":
		code &^= bitVal
	case "1", "(1)":
		code |= bitVal
	case "x":
		// unspecified bits, just ignore
	default:
		log.Fatalf("unexpected binary value %s in %s\n", value, inst.file)
	}
	return code
}

func (inst *InstructionParsed) setMask(code, bitVal uint32, value string) uint32 {
	switch value {
	// See the comment of [Regdiagram.mask]
	case "0", "1", "(0)", "(1)":
		code |= bitVal
	case "x":
		// unspecified bits, just ignore
	default:
		log.Fatalf("unexpected mask value %s in %s\n", value, inst.file)
	}
	return code
}

func (inst *InstructionParsed) boxEncoding(b Box, callBack func(uint32, uint32, string) uint32) uint32 {
	code := uint32(0)
	hi, err := strconv.Atoi(b.HiBit)
	if err != nil {
		log.Fatalf("convert HiBit to int failed, HiBit = %s in %s\n", b.HiBit, inst.file)
	}
	for _, c := range b.Cs {
		if c.ColSpan != "" {
			log.Fatalf("unexpected colspan in %s\n", inst.file)
		}
		code = callBack(code, uint32(1<<hi), c.Value)
		hi--
	}
	return code
}

func (inst *InstructionParsed) supported() bool {
	foundSVE := false
	for _, doc := range inst.DocVars {
		if doc.Key == "instr-class" {
			if doc.Value == "sve" || doc.Value == "sve2" {
				foundSVE = true
			}
		}
	}
	return foundSVE
}

// extractBinary extracts the known bits of instruction encoding in regdiagram,
// and assign the binary to inst.regdiagram.binary.
func (inst *InstructionParsed) extractBinary() {
	if !inst.supported() {
		return
	}
	for i := range inst.Classes.Iclass {
		bin, mask := uint32(0), uint32(0)
		regDiagram := &inst.Classes.Iclass[i].RegDiagram
		for _, box := range regDiagram.Boxes {
			if len(box.Cs) > 1 || (len(box.Cs) == 1 && box.Cs[0].ColSpan == "") {
				// Fixed bits
				bin |= inst.boxEncoding(box, inst.setBinary)
				mask |= inst.boxEncoding(box, inst.setMask)
			} else if len(box.Cs) == 1 && box.Cs[0].ColSpan != "" {
				// Named variable bits
				h, err := strconv.Atoi(box.HiBit)
				h++ // Arm provided high bit to be inclusive, but we need to make it exclusive.
				if err != nil {
					log.Fatalf("convert HiBit to int failed, HiBit = %s in %s\n", box.HiBit, inst.file)
				}
				cs, err := strconv.Atoi(box.Cs[0].ColSpan)
				if err != nil {
					log.Fatalf("convert ColSpan to int failed, ColSpan = %s in %s\n", box.Cs[0].ColSpan, inst.file)
				}
				if box.Name == "" {
					log.Fatalf("empty name in named box in %s\n", inst.file)
				}
				if regDiagram.varBin == nil {
					regDiagram.varBin = make(map[string]bitRange)
				}
				if _, ok := regDiagram.varBin[box.Name]; ok {
					log.Fatalf("duplicate name in named box in %s\n", inst.file)
				}
				regDiagram.varBin[box.Name] = bitRange{
					hi: h,
					lo: h - cs,
				}
			} else {
				log.Fatalf("unrecognized box in %s\n", inst.file)
			}
		}
		regDiagram.fixedBin = bin
		regDiagram.mask = mask
		regDiagram.Parsed = true
		if inst.Title == "URSQRTE -- A64" || inst.Title == "URECPE -- A64" {
			// Special case, its "size" box is actually not specified in the assembler symbol section.
			// By reading the decoding ASL we know that this "size" box should be 0b10...
			regDiagram.fixedBin |= uint32(1 << 23)
		}
		if len(inst.Classes.Iclass[i].PsSection) == 1 {
			squashedPs := inst.Classes.Iclass[i].PsSection[0].Ps[0].PSText
			if strings.Contains(strings.Join(squashedPs, "\n"), "if size IN {'0x'} then EndOfDecode") {
				// Very ugly encoding specification in the decoding ASL. We have to set
				// the high bit of the size box to 1.
				// Example instruction is "Unsigned divide (predicated)":
				// UDIV <Zdn>.<T>, <Pg>/M, <Zdn>.<T>, <Zm>.<T>
				if _, ok := regDiagram.varBin["size"]; !ok {
					log.Fatalf("size box not found in %s", inst.file)
				}
				if regDiagram.varBin["size"].hi != 24 || regDiagram.varBin["size"].lo != 22 {
					log.Fatalf("unexpecetd size box in %s", inst.file)
				}
				regDiagram.fixedBin |= uint32(1 << 23)
			}
		}
	}
}

// processEncoding handles each encoding element of a inst.
func (inst *InstructionParsed) processEncodings() {
	if !inst.supported() {
		return
	}
	for i := range inst.Classes.Iclass {
		iclass := &inst.Classes.Iclass[i]
		for j := 0; j < len(iclass.Encodings); j++ {
			enc := &iclass.Encodings[j]
			// Set instruction class.
			if !enc.instClass() {
				// Unsupported instruction class.
				continue
			}
			// Set alias
			enc.Alias = inst.Type == "alias"
			// Refine the known bits and mask of the binary.
			bin, mask := iclass.RegDiagram.fixedBin, iclass.RegDiagram.mask
			for _, box := range enc.Boxes {
				bin |= inst.boxEncoding(box, inst.setBinary)
				mask |= inst.boxEncoding(box, inst.setMask)
			}
			enc.Binary = bin
			enc.mask = mask

			inst.parseOperands(enc)
			inst.arm64Opcode(enc)
			inst.goOpcode(enc)
			inst.template(enc)
			inst.operandsType(enc)
			enc.sortOperands()
			if inst.Title == "REVB, REVH, REVW -- A64" ||
				inst.Title == "SXTB, SXTH, SXTW -- A64" ||
				inst.Title == "UXTB, UXTH, UXTW -- A64" {
				// Special case, its "size" box is not specified in the assembler symbol
				// section for the [B] and [W] variants, which for [B] it's 0b00 (no-op)
				// and for [W] it's 0b11.
				imnemonic := enc.Operands[0].Name
				if imnemonic[len(imnemonic)-1] == 'W' {
					enc.Binary |= uint32(0b11 << 22)
				}
			}
			enc.Parsed = true
		}
	}
}

func (inst *InstructionParsed) findExplanation(link string) *Explanation {
	for _, exp := range inst.Explanations.Explanations {
		if exp.Symbol.Link == link {
			return &exp
		}
	}
	return nil
}

func trimXMLEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&lt;", "<"), "&gt;", ">")
}

// key is the fixed symbol, value is its attached symbols (the currently parsed part of the operand)
// and the instruction that contains this fixed symbol.
// This data is just used for debugging.
var allFixedSymbolsLock sync.Mutex
var allFixedSymbols = map[string]map[string]*InstructionParsed{}

func (inst *InstructionParsed) parseOperands(enc *EncodingParsed) {
	// This is the most vulnerable part.
	//
	// The mnemonic and operands of an instruction are sequentially recorded
	// in TextA, and we need to parse them out. According to the following rules:
	// 1. The mnemonic and the operand are separated by " ".
	// 2. The operands are separated by ", ". Symbols without intervals belong to
	//    the same operand.
	// 3, An operand may contain [] and {}, and the brackets contained in the
	//    operand must be in pairs. For example <R><m>{, <extend> {#<amount>}}
	//    is one operand, not two.
	//
	// After this step we'll get all operands of this instruction encoding, the
	// operand interval symbol ", " will be discarded.
	asm, oprAsm := "", ""
	leftCurly, leftSquare := 0, 0
	elems := []Element{}
	recordFixedSymbol := func(symbol, val string) {
		allFixedSymbolsLock.Lock()
		if _, ok := allFixedSymbols[symbol]; !ok {
			allFixedSymbols[symbol] = make(map[string]*InstructionParsed)
		}
		if _, ok := allFixedSymbols[symbol][val]; ok {
			allFixedSymbolsLock.Unlock()
			return
		}
		allFixedSymbols[symbol][val] = inst
		allFixedSymbolsLock.Unlock()
	}

	for m, ta := range enc.AsmTemplate.TextA {
		val := ta.Value
		if link := ta.Link; link != "" { // An <a> element
			// Check if it's a special operand with fixed candidates.
			exp := inst.findExplanation(link)
			if exp == nil {
				log.Fatalf("explanation not found for link %s in %s\n", link, inst.file)
			}
			var explanation, encodedin string
			if tblClass := exp.Definition.Table.Class; tblClass != "" {
				switch tblClass {
				case "valuetable":
					heads := []string{}
					for _, entry := range exp.Definition.Table.TGroup.THead.Row.Entries {
						heads = append(heads, trimXMLEscape(entry.Value))
					}
					bodies := [][]string{}
					for i, row := range exp.Definition.Table.TGroup.TBody.Row {
						bodies = append(bodies, []string{})
						for _, entry := range row.Entries {
							bodies[i] = append(bodies[i], entry.Value)
						}
					}
					explanation = strings.Join(heads, "\t")
					for _, body := range bodies {
						explanation += "\n" + strings.Join(body, "\t")
					}
					if explanation == "" {
						log.Fatalf("explanation is empty in %s\n", inst.file)
					}
					explanation = trimXMLEscape(exp.Definition.Intro) + "\n" + explanation
					encodedin = exp.Definition.Encodedin
					if encodedin == "" {
						log.Fatalf("definition.encodedin is empty in %s\n", inst.file)
					}
				default:
					log.Fatalf("unknown table class %s in %s\n", tblClass, inst.file)
				}
			} else if exp.Account.Encodedin != "" {
				explanation = trimXMLEscape(exp.Account.Intro)
				if explanation == "" {
					log.Fatalf("account.intro.para is empty in %s\n", inst.file)
				}
				encodedin = exp.Account.Encodedin
				if encodedin == "" {
					log.Fatalf("account.encodedin is empty in %s\n", inst.file)
				}
			}
			val = trimXMLEscape(val)
			elem := Element{encodedIn: encodedin, textExp: explanation, symbol: val}
			// Some hardcoded logic to populate register type and the presence
			// of <mod> for deduplication purposes
			if strings.HasPrefix(val, "<X") {
				elem.fixedScalarWidth = 64
				recordFixedSymbol("X", val)
			} else if strings.HasPrefix(val, "<W") {
				elem.fixedScalarWidth = 32
				recordFixedSymbol("W", val)
			} else if val == "<mod>" {
				recordFixedSymbol("mod", val)
				elem.hasMod = true
			} else if strings.HasPrefix(val, "<P") {
				recordFixedSymbol("P", val)
				elem.isP = true
			} else if strings.HasPrefix(val, "<Z") {
				recordFixedSymbol("Z", val)
				elem.isZ = true
			}
			elems = append(elems, elem)
		} else {
			// It's a text section, we want to extract fixed symbols if any.
			for _, rule := range fixedElemRules {
				matchCnt := 0
				if rule.re.MatchString(val) {
					if len(elems) == 0 {
						// These instructions are just named UXTW and SXTW
						if rule.t == FixedUXTW && inst.Title == "UXTB, UXTH, UXTW -- A64" {
							continue
						}
						if rule.t == FixedSXTW && inst.Title == "SXTB, SXTH, SXTW -- A64" {
							continue
						}
						log.Fatalf("fixed arrangement symbol %s without preceding element in %s\n", val, inst.file)
					}
					if matchCnt != 0 {
						log.Fatalf("fixed arrangement symbol %s with multiple match in %s\n", val, inst.file)
					}
					matchCnt++
					lastElem := &elems[len(elems)-1]
					switch rule.t {
					case FixedArrangement:
						lastElem.fixedArng = rule.val
					case FixedLSL:
						lastElem.fixedLSL = rule.val
					case FixedSXTW:
						lastElem.fixedSXTW = true
					case FixedUXTW:
						lastElem.fixedUXTW = true
					case FixedModAmt:
						if lastElem.symbol == "<mod>" {
							lastElem.fixedModAmt = rule.val
						}
					}
					// Also book keep the fixed symbol in the global map.
					recordFixedSymbol(rule.val, fmt.Sprintf("%s in %s", lastElem.symbol, oprAsm+val))
				}
			}
		}
		asm += val

		appendOperand := func() {
			elemsCopy := make([]Element, len(elems))
			copy(elemsCopy, elems)
			opr := Operand{Name: oprAsm, Elems: elemsCopy}
			enc.Operands = append(enc.Operands, opr)
			oprAsm = ""
			elems = elems[:0]
		}

		// Parse operands
		for n := 0; n < len(val); n++ {
			ch := val[n]
			switch ch {
			case ',':
				if leftCurly == 0 && leftSquare == 0 {
					// This "," is an interval.
					continue
				}
			case ' ':
				if leftCurly == 0 && leftSquare == 0 {
					if oprAsm == "" {
						// Consecutive space separators.
						continue
					}
					// This first one is mnemonic, followed by operands.
					appendOperand()
					continue
				}
			case '{':
				leftCurly++
			case '[':
				leftSquare++
			case '}':
				leftCurly--
			case ']':
				leftSquare--
			}
			oprAsm += string(ch)
		}
		// The last operand.
		if m == len(enc.AsmTemplate.TextA)-1 && leftCurly == 0 && leftSquare == 0 && oprAsm != "" {
			appendOperand()
		}
	}
	if oprAsm != "" || len(elems) != 0 {
		log.Fatalf("malformed Asmtemplate, oprAsm: %v, elems: %v in %s\n", oprAsm, elems, inst.file)
	}
	enc.Asm = asm
}

// template resets the arm64 assembly template of an encoding, to make it cleaner.
func (inst *InstructionParsed) template(enc *EncodingParsed) {
	asm := enc.Operands[0].Name
	if len(enc.Operands) > 1 { // Has operands
		asm += "  "
		i := 1
		for ; i < len(enc.Operands)-1; i++ {
			asm += enc.Operands[i].Name + ", "
		}
		asm += enc.Operands[i].Name
	}
	enc.Asm = asm
}

// arm64Opcode sets the arm64 opcode of an encoding.
func (inst *InstructionParsed) arm64Opcode(enc *EncodingParsed) {
	if len(enc.Operands) == 0 {
		log.Fatalf("Miss mnemonic: %v in %s\n", enc, inst.file)
	}
	// Add a prefix "A64", to differ with the "A" prefix of Go opcode.
	enc.arm64Op = "A64" + enc.Operands[0].Name
}

func (enc *EncodingParsed) classString() string {
	val := ""
	for _, d := range enc.DocVars {
		if d.Key == "instr-class" {
			val = d.Value
			break
		}
	}
	return val
}

func (enc *EncodingParsed) instClass() bool {
	val := enc.classString()
	switch val {
	case "sve":
		enc.class = C_SVE
	case "sve2":
		enc.class = C_SVE2
	default:
		return false
	}
	return true
}

func (enc *EncodingParsed) hasZREG() bool {
	// Special case: <Pg>/<ZM>, <ZM> is not Z register.
	return reZREG.MatchString(enc.Asm)
}

func (enc *EncodingParsed) hasPREG() bool {
	return rePREG.MatchString(enc.Asm)
}

func (enc *EncodingParsed) goOpcodePrefix(inst *InstructionParsed) string {
	if enc.prefix != "" {
		return enc.prefix
	}
	prefix := ""
	switch enc.class {
	case C_SVE, C_SVE2:
		if enc.hasZREG() {
			prefix = "Z"
		} else if enc.hasPREG() {
			prefix = "P"
		}
	default:
		log.Fatalf("unknown instruction class %v in %s\n", enc.class, inst.file)
	}
	return prefix
}

// goOpcode determines the Go opcode representation of an encoding.
func (inst *InstructionParsed) goOpcode(enc *EncodingParsed) {
	if len(enc.Operands) == 0 {
		log.Fatalf("Missing mnemonic: %v in %s\n", enc, inst.file)
	}
	if enc.GoOp != "" {
		return
	}
	prefix, opcode := "A", ""
	prefix += enc.goOpcodePrefix(inst)
	opcode = enc.Operands[0].Name
	enc.GoOp = prefix + opcode
	enc.prefix = prefix
}

// sortOperands reorders the operands of an encoding according to Go assembly syntax.
func (enc *EncodingParsed) sortOperands() {
	// Reverse args, placing dest last.
	for i, j := 1, len(enc.Operands)-1; i < j; i, j = i+1, j-1 {
		enc.Operands[i], enc.Operands[j] = enc.Operands[j], enc.Operands[i]
	}
}

func (inst *InstructionParsed) operandType(opr Operand) string {
	if opr.Typ != "" {
		return opr.Typ
	}
	name := opr.Name
	for i := 0; i < len(operandRules); i++ {
		if operandRules[i].re.MatchString(name) {
			return operandRules[i].class
		}
	}
	inst.ParseError = fmt.Sprintf("unrecognized operand type: %s in %s\n", name, inst.file)
	return "AC_NONE"
}

// operandsType classifies all operands of an encoding.
func (inst *InstructionParsed) operandsType(enc *EncodingParsed) {
	for i := 1; i < len(enc.Operands); i++ {
		enc.Operands[i].Typ = inst.operandType(enc.Operands[i])
	}
}

func ProcessXMLFiles(insts []*InstructionParsed) {
	var wg sync.WaitGroup
	sort.Slice(insts, func(i, j int) bool {
		if insts[i] == nil {
			return false
		}
		if insts[j] == nil {
			return true
		}
		return insts[i].Title < insts[j].Title
	})
	for i, inst := range insts {
		if inst == nil {
			insts = insts[:i]
			break
		}
	}
	for _, inst := range insts {
		wg.Add(1)
		go func(inst *InstructionParsed) {
			defer wg.Done()
			inst.extractBinary()
			inst.processEncodings()
		}(inst)
	}
	wg.Wait()
	validate(insts)
	debugInfo(*debug)
}

// The operand constraints, the value is an example instruction.
var allOpConstraints = map[string]*InstructionParsed{}

// The encoding function descriptions with their references to named bit ranges expanded.
// The value is an example instruction.
var AllEncodingDescs = map[string]*InstructionParsed{}

// The mapping from encoding function description to encoded-in.
var EncodingDescsToEncodedIn = map[string]string{}
var concatedRangeRe = regexp.MustCompile(`\((.*?) :: (.*?)(?: :: (.*?))?\)`)
var rangeIndexRe = regexp.MustCompile(`(.*?)\[(\d+)\]`)

func (inst *InstructionParsed) expandNamedBitRanges(elm *Element, varBin map[string]bitRange) string {
	ranges := map[string]string{}
	textExp := elm.textExp
	br, ok := varBin[elm.encodedIn]
	if !ok {
		if matches := concatedRangeRe.FindStringSubmatch(elm.encodedIn); len(matches) > 1 {
			for _, key := range matches[1:] {
				br, ok2 := varBin[key]
				if ok2 {
					ranges[key] = fmt.Sprintf("[%d:%d)", br.lo, br.hi)
				}
				ok = true
			}
		} else if matches := rangeIndexRe.FindStringSubmatch(elm.encodedIn); len(matches) > 2 {
			key := matches[1]
			idx := matches[2]
			idxI, err := strconv.Atoi(idx)
			if err != nil {
				log.Fatalf("invalid index: %s in %s, available: %v in %s\n", idx, elm.encodedIn, varBin, inst.file)
			}
			br, ok2 := varBin[key]
			if ok2 {
				ok = true
				ranges[key] = fmt.Sprintf("[%d:%d)", br.lo+idxI, br.lo+idxI+1)
			}
		}
	} else {
		ranges[elm.encodedIn] = fmt.Sprintf("[%d:%d)", br.lo, br.hi)
	}
	if !ok {
		if inst.Title == "SDOT (4-way, vectors) -- A64" || inst.Title == "UDOT (4-way, vectors) -- A64" {
			// Known inconsistencies, the box contains a fixed bit, and the parsing logic missed it.
			ranges["size"] = "[22:23)"
			ok = true
		} else {
			log.Printf("unknown bit range: %s in %s, available: %v in %s\n", elm.encodedIn, elm.textExp, varBin, inst.file)
		}
	}
	textExp += "\nbit range mappings:\n"
	// Sort keys to ensure deterministic order
	keys := make([]string, 0, len(ranges))
	for k := range ranges {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		textExp += fmt.Sprintf("%s: %s\n", k, ranges[k])
	}
	return textExp
}

// validate does the following:
// 1. checks if all instruction encodings are unique with regard to this tuple:
//
//	(assembly mnemonic, [operand info])
//
// Note: variable arrangements are not checked, as before we reason about the encoding
// semantics we cannot fully deduplicate them, e.g.:
//
//	SDOT  <Zda>.<T>, <Zn>.<Tb>, <Zm>.<Tb>
//	SDOT  <Zda>.H, <Zn>.B, <Zm>.B
//	SDOT  <Zda>.S, <Zn>.H, <Zm>.H
//
// <T> and <Tb> are specified in the encoding text, that there is a constraint "T = 4*Tb".
// We don't know this fact by looking at the <asmtemplate> solely, without this information
// the first encoding domain entails the rest 2.
// We defer this deduplication to the assembler.
//
// 2. populates the constraints field of each operand.
// 3. bookkeep the encoding function descriptions and operand constraints.
func validate(insts []*InstructionParsed) {
	allEncodings := map[string][]string{}
	for _, inst := range insts {
		for i, iclass := range inst.Classes.Iclass {
			for j, encoding := range iclass.Encodings {
				if encoding.Parsed == false {
					continue
				}
				key := encoding.arm64Op
				for k, operand := range encoding.Operands {
					key += " " + operand.Typ
					for l, elem := range operand.Elems {
						constraints := []string{fmt.Sprintf("COP_%s__%d_", operand.Typ, l)}
						if elem.fixedArng != "" {
							key += "_(Arng:" + elem.fixedArng + ")"
							constraints = append(constraints, "ARNG"+elem.fixedArng)
						}
						if elem.fixedModAmt != "" {
							key += "_(ModAmt:" + elem.fixedModAmt + ")"
							constraints = append(constraints, "MODAMT"+elem.fixedModAmt)
						}
						if elem.fixedScalarWidth != 0 {
							key += fmt.Sprintf("_(ScalarWidth:%d)", elem.fixedScalarWidth)
							constraints = append(constraints, fmt.Sprintf("R%d", elem.fixedScalarWidth))
						}
						if elem.fixedLSL != "" {
							key += "_(LSL:" + elem.fixedLSL + ")"
							constraints = append(constraints, "LSL"+elem.fixedLSL)
						}
						if elem.fixedSXTW {
							key += "_(SXTW)"
							constraints = append(constraints, "SXTW")
						}
						if elem.fixedUXTW {
							key += "_(UXTW)"
							constraints = append(constraints, "UXTW")
						}
						if elem.hasMod {
							key += "_(mod)"
						}
						if elem.isP {
							key += "_(P)"
						}
						if elem.isZ {
							key += "_(Z)"
						}
						var cStr = "COP_NONE"
						if len(constraints) > 1 {
							cStr = strings.Join(constraints, "_")
							allOpConstraints[cStr] = inst
						}
						inst.Classes.Iclass[i].Encodings[j].Operands[k].constraints = append(
							inst.Classes.Iclass[i].Encodings[j].Operands[k].constraints, cStr)
						textExpWithRanges := inst.expandNamedBitRanges(&elem, iclass.RegDiagram.varBin)
						AllEncodingDescs[textExpWithRanges] = inst
						if existing, ok := EncodingDescsToEncodedIn[textExpWithRanges]; ok && existing != elem.encodedIn {
							log.Fatalf("duplicate encoding description for two different encoded-ins: %s for %s and %s in %s\n",
								textExpWithRanges, existing, elem.encodedIn, inst.file)
						}
						EncodingDescsToEncodedIn[textExpWithRanges] = elem.encodedIn
						inst.Classes.Iclass[i].Encodings[j].Operands[k].Elems[l].TextExpWithRanges = textExpWithRanges
					}
					inst.Classes.Iclass[i].Encodings[j].Operands[k].resolveConstraints()
				}
				allEncodings[key] = append(allEncodings[key], encoding.Asm)
			}
		}
	}
	keys := make([]string, 0, len(allEncodings))
	for k := range allEncodings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := allEncodings[k]
		if len(v) > 1 {
			if strings.HasPrefix(k, "A64MOV") {
				// These currently are:
				//	MOV  <Zd>.<T>, #<const> (mov_dupm_z_i.xml)
				//	MOV  <Zd>.<T>, #<imm>{, <shift>} (mov_dup_z_i.xml)
				// These 2 instructions actually overlaps their domain!
				// Although <shift> is optional here, we can force the user
				// to always specify <shift> to manually deduplicate.
				// Otherwise the assembler will just panic.
				continue
			}
			if strings.HasPrefix(k, "A64FMOV") || strings.HasPrefix(k, "A64COMPACT") {
				// Their domains does not overlap, it's ok to ignore.
				continue
			}
			if len(v) == 2 {
				var z2Cnt, z4Cnt int
				for _, s := range v {
					if strings.Contains(s, "-<Zt2>") || strings.Contains(s, "-<Zdn2>") {
						z2Cnt++
					}
					if strings.Contains(s, "-<Zt4>") || strings.Contains(s, "-<Zdn4>") {
						z4Cnt++
					}
				}
				// These are reglists of 2 or 4 registers, they will be deduplicated by the assembler
				// at encoding stage.
				if z2Cnt == 1 && z4Cnt == 1 {
					continue
				}
			}
			// If the diff is only by Pg/M or Pg/Z, it's ok to ignore, they are handled by the assembler.
			if len(v) == 2 {
				var hasPgM, hasPgZ bool
				for _, s := range v {
					if strings.Contains(s, "/M") {
						hasPgM = true
					}
					if strings.Contains(s, "/Z") {
						hasPgZ = true
					}
				}
				if hasPgM && hasPgZ {
					continue
				}
			}
			sort.Strings(v)
			log.Printf("%s:\n\t%v\n", k, strings.Join(v, "\n\t"))
		}
	}
}

// expectedElemCount is the expected number of elements for each
// operand class (AClass in the assembler).
// The comments on the map elems are the GNU mnemonic forms;
// the arrow-bracket enclosed parts are elements.
var expectedElemCount = map[string]int{
	// <reg>.<T>
	"AC_ARNG":   2,
	"AC_PREG":   2,
	"AC_PREGZM": 2,
	"AC_ZREG":   2,
	// <reg>.<T>[<index>]
	"AC_ARNGIDX": 3,
	"AC_ZREGIDX": 3,
	"AC_REGIDX":  3,
	// #<imm>, <shift>
	"AC_IMM": 2,
	// [<reg1>.<T1>, <reg2>.<T2>, <mod> <amount>]
	"AC_MEMEXT": 6,
	// [<reg>.<T>, #<imm>]
	"AC_MEMOFF": 3,
	// [<xn|sp>{, #<imm>, MUL VL}]
	// xn implies the constraint that it's an X reg, so one additional encoding func to check.
	"AC_MEMOFFMULVL": 3,
	// <preg>.<T>[<selreg>, <imm>]
	// selreg must be a W reg, so one additional encoding func to check.
	"AC_PREGIDX": 5,
	// <width><reg>
	"AC_REG": 2,
	"AC_RSP": 2,
	// {<reg>.<T>}
	"AC_REGLIST1": 2,
	// {<reg1>.<T1>, <reg2>.<T2>}
	"AC_REGLIST2": 4,
	// {<reg1>.<T1>, <reg2>.<T2>, <reg3>.<T3>}
	"AC_REGLIST3": 6,
	// {<reg1>.<T1>, <reg2>.<T2>, <reg3>.<T3>, <reg4>.<T4>}
	"AC_REGLIST4": 8,
	// {<reg1>.<T1>-<reg2>.<T2>}
	"AC_REGLIST_RANGE": 4,
	// <vl> or <prfop>
	"AC_SPECIAL": 1,
	// <arrangement><reg>
	"AC_VREG": 2,
	// {<Reg>.<T>, <pattern>, MUL #<imm>}
	"AC_PREG_PATTERN": 4,
	"AC_REG_PATTERN":  4,
	"AC_ZREG_PATTERN": 4,
}

// unresolvedConstraints stores the constraints that are not resolved.
var unresolvedConstraints = map[string]struct{}{}

// resolveConstraints resolves the constraints for the given operand,
// It understands the logic and expands the constraints to encoding functions
// with proper comments. Resolving constraints is done by replacing the
// constrained element with more elements in place, and adding the constraints
// to the new element's encoding function.
// This function also checks that the operand has the expected number of elements
// after resolving the constraints.
func (op *Operand) resolveConstraints() {
	insertElmAt := func(idx int, symbol, textExpWithRanges string) {
		op.Elems = append(op.Elems[:idx], append([]Element{
			{
				encodedIn:         "nil",
				TextExpWithRanges: textExpWithRanges,
				symbol:            symbol,
			},
		}, op.Elems[idx:]...)...)
		AllEncodingDescs[textExpWithRanges] = nil
	}
	// Constraint format: COP_<AClass>__<index>_(_<constraintTypes>)*
	// <AClass> is the operand class, e.g. AC_REG, AC_IMM, etc.
	// <index> is the index of the operand in the instruction, e.g. 0, 1, 2, etc.
	// <constraintTypes> is the type of the constraint, e.g. ARNG, MODAMT, etc.
	for _, constraint := range op.constraints {
		constraint = strings.TrimPrefix(constraint, "COP_")
		parts := strings.Split(constraint, "__")
		if len(parts) != 3 {
			if constraint != "NONE" {
				log.Printf("Invalid constraint format: %s", constraint)
			}
			continue
		}
		acl := parts[0]
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("Invalid constraint format: %s", constraint)
		}
		constraintTypes := strings.Split(parts[2], "_")
		for _, constraintType := range constraintTypes {
			switch constraintType {
			case "ARNGB":
				insertElmAt(index+1, "B", "Check this is a B arrangement")
				index++
			case "ARNGD":
				insertElmAt(index+1, "D", "Check this is a D arrangement")
				index++
			case "ARNGH":
				insertElmAt(index+1, "H", "Check this is a H arrangement")
				index++
			case "ARNGQ":
				insertElmAt(index+1, "Q", "Check this is a Q arrangement")
				index++
			case "ARNGS":
				insertElmAt(index+1, "S", "Check this is a S arrangement")
				index++
			case "R64":
				insertElmAt(index+1, "X", "Check this is a 64-bit scalar register")
				index++
			case "R32":
				insertElmAt(index+1, "W", "Check this is a 32-bit scalar register")
				index++
			case "LSL1", "LSL2", "LSL3", "LSL4", "SXTW", "UXTW", "MODAMT1", "MODAMT2", "MODAMT3":
				if acl == "AC_MEMEXT" {
					switch constraintType {
					case "LSL1", "LSL2", "LSL3", "LSL4":
						insertElmAt(index+1, "LSL", "Check this is mod and is LSL")
						index++
					case "UXTW":
						insertElmAt(index+1, "UXTW", "Check this is mod and is UXTW")
						index++
					case "SXTW":
						insertElmAt(index+1, "SXTW", "Check this is mod and is SXTW")
						index++
					}
					switch constraintType {
					case "LSL1", "MODAMT1":
						insertElmAt(index+1, "#1", "Check this is mod amount and is 1")
						index++
					case "LSL2", "MODAMT2":
						insertElmAt(index+1, "#2", "Check this is mod amount and is 2")
						index++
					case "LSL3", "MODAMT3":
						insertElmAt(index+1, "#3", "Check this is mod amount and is 3")
						index++
					case "LSL4":
						insertElmAt(index+1, "#4", "Check this is mod amount and is 4")
						index++
					}
				} else {
					log.Printf("Unknown constraint: %s", constraint)
				}
			default:
				log.Printf("Unknown constraint: %s", constraint)
			}
		}
	}
	noOpCheck := "No-op check, returns true"
	// Check the number of elements
	if el := expectedElemCount[op.Typ]; len(op.Elems) != el {
		resolved := false
		switch op.Name {
		case "#0.0":
			if el == 2 && len(op.Elems) == 0 {
				op.Elems = make([]Element, 0, 2)
				insertElmAt(0, "#0.0", "Check this is immediate 0.0")
				insertElmAt(1, "nil", noOpCheck)
				resolved = true
			}
		case "#<const>", "#<imm1>", "#<imm2>", "#<imm>", "<const>":
			if el == 2 && len(op.Elems) == 1 {
				insertElmAt(1, "nil", noOpCheck)
				resolved = true
			}
		case "<Dd>", "<Pd>", "<Pg>", "<Pn>", "<PNg>", "<Pt>", "<Pv>", "<Zd>", "<Zm>", "<Zn>", "<Zt>":
			if el == 2 && len(op.Elems) == 1 {
				insertElmAt(1, "nil", noOpCheck)
				resolved = true
			}
		case "<PNg>/Z", "<Pg>/Z":
			if el == 2 && len(op.Elems) == 1 {
				insertElmAt(1, "Z", "Check this is a zeroing predication")
				resolved = true
			}
		case "<Pg>/M", "<Pv>/M":
			if el == 2 && len(op.Elems) == 1 {
				insertElmAt(1, "M", "Check this is a merging predication")
				resolved = true
			}
		case "<PNn>[<imm>]":
			if el == 3 && len(op.Elems) == 2 {
				insertElmAt(1, "nil", noOpCheck)
				resolved = true
			}
		case "<Pd>.<T>{, <pattern>}":
			if el == 4 && len(op.Elems) == 3 {
				insertElmAt(3, "nil", noOpCheck)
				resolved = true
			}
		case "<Zd>{[<imm>]}", "<Zm>[<index>]", "<Zn>{[<imm>]}":
			if el == 3 && len(op.Elems) == 2 {
				insertElmAt(1, "nil", noOpCheck)
				resolved = true
			}
		case "[<Xn|SP>, <Xm>]", "[<Xn|SP>, <Zm>.D]", "[<Xn|SP>{, <Xm>}]", "[<Zn>.D{, <Xm>}]", "[<Zn>.S{, <Xm>}]":
			if el == 6 && len(op.Elems) == 4 {
				insertElmAt(4, "nil", noOpCheck)
				insertElmAt(5, "nil", noOpCheck)
				resolved = true
			}
		case "[<Xn|SP>, <Zm>.S, <mod>]", "[<Xn|SP>, <Zm>.D, <mod>]":
			if el == 6 && len(op.Elems) == 5 {
				insertElmAt(5, "nil", noOpCheck)
				resolved = true
			}
		}
		if !resolved {
			unresolvedConstraints[fmt.Sprintf("Operand %s has %d elements, expected %d", op.Name, len(op.Elems), expectedElemCount[op.Typ])] = struct{}{}
		}
	}
}

// debugInfo prints all fixed symbols, operand constraints and encoding function descriptions
// in deterministic order.
func debugInfo(debug int) {
	log.Printf("len(allFixedSymbols) = %v\n", len(allFixedSymbols))
	if debug > 0 {
		keys := make([]string, 0, len(allFixedSymbols))
		for k := range allFixedSymbols {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s:\n", k)
			for v2 := range allFixedSymbols[k] {
				fmt.Printf("\t%s\n", v2)
				if debug > 1 {
					fmt.Printf("Example Inst at %s\n", allFixedSymbols[k][v2].file)
				}
			}
		}
	}
	log.Printf("len(allOpConstraints) = %v\n", len(allOpConstraints))
	if debug > 0 {
		keys := make([]string, 0, len(allOpConstraints))
		for k := range allOpConstraints {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s\n", k)
			if debug > 1 {
				fmt.Printf("Example Inst at %s\n", allOpConstraints[k].file)
			}
		}
	}
	log.Printf("len(AllEncodingDescs) = %v\n", len(AllEncodingDescs))
	if debug > 0 {
		keys := make([]string, 0, len(AllEncodingDescs))
		for k := range AllEncodingDescs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s\n", k)
			if debug > 1 {
				fmt.Printf("Example Inst at %s\n", AllEncodingDescs[k].file)
			}
		}
	}
	keys := make([]string, 0, len(unresolvedConstraints))
	for k := range unresolvedConstraints {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf("%s\n", k)
	}
}
