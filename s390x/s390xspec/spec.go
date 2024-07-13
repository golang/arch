// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S390xspec reads the Principles of Operation PDF Manual
// to collect instruction encoding details and writes those details to standard output
// in CSV format.
//
// Usage:
//
//	s390xspec z_Architecture_Principles_of_Operation.pdf > s390x.csv
//
// Each CSV line contains three fields:
//
//	instruction
//		The instruction heading, such as "BRANCH AND LINK".
//	mnemonic
//		The instruction mnemonics, such as "BAL R1,D2(X2,B2)".
//	encoding
//		The instruction encoding, a sequence of opcode and operands encoding in respective bit positions
//		such as operand@bitposition each separated by |
//		Ex: "45@0|R1@8|X2@12|B2@16|D2@20|"
//
// For more on the exact meaning of these fields, see the Principle of Operations IBM-Z Architecture PDF Manual.
package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"rsc.io/pdf"
	"sort"
	"strconv"
	"strings"
)

type Inst struct {
	Name  string
	Text  string
	Enc   string
	Flags string
}

var stdout *bufio.Writer

func main() {
	log.SetFlags(0)
	log.SetPrefix("s390xspec: ")

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: s390xspec file.pdf\n")
		os.Exit(2)
	}

	f, err := pdf.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// Split across multiple columns and pages!
	var all = []Inst{}

	// Scan document looking for instructions.
	// Must find exactly the ones in the outline.
	n := f.NumPage()
	for pageNum := 1; pageNum <= n; pageNum++ {
		page := f.Page(pageNum)
		t1 := getPageContent(page)
		if len(t1) > 0 && match(t1[0], "Helvetica-Bold", 13.98, "Instructions Arranged by Name") {
			for n := pageNum; n < pageNum+24; n++ {
				page := f.Page(n)
				table := parsePage(n, page)
				all = append(all, table...)
			}
			break
		} else {
			continue
		}
	}
	stdout = bufio.NewWriter(os.Stdout)
	for _, inst := range all {
		if strings.Contains(inst.Name, "\x00I") {
			r := rune(0x2190)
			inst.Name = strings.Replace(inst.Name, "\x00I", string(r), -1)
		} else if strings.Contains(inst.Name, "I\x00") {
			r := rune(0x2192)
			inst.Name = strings.Replace(inst.Name, "I\x00", string(r), -1)
		}
		fmt.Fprintf(stdout, "%q,%q,%q,%q\n", inst.Name, inst.Text, inst.Enc, inst.Flags)
	}
	stdout.Flush()

}

// getPageContent gets the page content of a single PDF page
func getPageContent(p pdf.Page) []pdf.Text {
	var text []pdf.Text

	content := p.Content()
	for _, t := range content.Text {
		text = append(text, t)
	}

	text = findWords(text)
	return text
}

// parsePage parses single PDF page and returns the instructions content
func parsePage(num int, p pdf.Page) []Inst {
	var insts []Inst
	text := getPageContent(p)

	for {
		var heading, mnemonic, format string
		// The float numbers below are the horizontal X-coordinate values to be parsed out of the Z-ISA PDF book.
		for len(text) > 0 && !(match(text[0], "Helvetica-Narrow", 8, "") && (matchXCord(text[0], 73.9) || matchXCord(text[0], 55.9))) {
			text = text[1:]
		}
		if len(text) == 0 {
			break
		}
		heading = text[0].S
		text = text[1:]
		// The float numbers below are the horizontal X-coordinate values to be parsed out of the Z-ISA PDF book.
		for !(matchXCord(text[0], 212.2) || matchXCord(text[0], 230.1) || matchXCord(text[0], 246.2) || matchXCord(text[0], 264.2)) {
			heading += text[0].S
			if match(text[0], "Wingdings3", 0, "") {
				heading += text[1].S
				text = text[1:]
			}
			text = text[1:]
		}
		if strings.Compare(heading, "DIAGNOSE") == 0 {
			text = text[1:]
			continue
		}
		heading, check, m := checkHeading(heading)
		if check {
			mnemonic = m
		} else {
			mnemonic = text[0].S
			text = text[1:]
		}
		index := strings.Index(mnemonic, " ")
		if index != -1 {
			format = mnemonic[index+1:]
			mnemonic = mnemonic[:index]
		} else {
			format = text[0].S
		}
		text = text[1:]
		if strings.Compare(format, "SS") == 0 {
			format += text[0].S
		}
		before, _, _ := strings.Cut(format, " ")
		format = before
		// The float numbers below are the horizontal X-coordinate values to be parsed out of the Z-ISA PDF book.
		for len(text) > 0 && !(match(text[0], "Helvetica-Narrow", 8, "") && (matchXCord(text[0], 350.82) || matchXCord(text[0], 363.84) || matchXCord(text[0], 332.82) || matchXCord(text[0], 345.84))) {
			if text[0].X > 405.48 {
				break
			}
			text = text[1:]
		}
		flags := text[0].S
		// The float numbers below are the horizontal X-coordinate values to be parsed out of the Z-ISA PDF book.
		for len(text) > 0 && !(match(text[0], "Helvetica-Narrow", 8, "") && ((matchXCord(text[0], 481.7) && (!matchXCord(text[1], 496.1))) || matchXCord(text[0], 496.1) || (matchXCord(text[0], 499.6) && (!matchXCord(text[1], 514))) || (matchXCord(text[0], 514)))) {
			text = text[1:]
		}
		if len(text) == 0 {
			break
		}
		opcode := text[0].S
		b1, b2, _ := strings.Cut(opcode, " ")
		if matchXCord(text[0], 481.7) || matchXCord(text[0], 499.6) {
			opcode = b2
		} else {
			opcode = b1
		}
		if strings.Compare(text[0].S, b1) == 0 {
			text = text[2:]
		} else {
			text = text[1:]
		}
		mnemonic1, encoding := frameMnemonic(mnemonic, format, opcode)
		for match(text[0], "Helvetica-Narrow", 5.1, "") {
			text = text[1:]
		}
		if match(text[0], "Helvetica-Oblique", 9, "") {
			text = text[2:]
			insts = append(insts, Inst{heading, mnemonic1, encoding, flags})
			continue
		}
		if strings.HasPrefix(text[0].S, "(") {
			y123 := text[0].Y
			for text[0].Y == y123 && !matchXCord(text[0], 5.1) {
				heading += text[0].S
				text = text[1:]
			}
		} else if !(math.Abs(text[0].Y-text[1].Y) < 0.3) {
			heading += " " + text[0].S
			text = text[1:]
		}
		insts = append(insts, Inst{heading, mnemonic1, encoding, flags})
		if match(text[0], "Helvetica-Oblique", 9, "") {
			break
		}
	}
	return insts
}

func checkHeading(heading string) (string, bool, string) {
	substr := []string{"ALSI", "ALGSI", "CHRL", "CGHRL", "CUXTR", "IEXTR", "RXSBG", "RISBLG", "VERIM", "VPSOP"}
	b := false
	for _, s := range substr {
		r1 := strings.Index(heading, s)
		if r1 != -1 {
			heading = heading[:r1-1]
			b = true
			return heading, b, s
		}
	}
	return heading, b, ""
}

func frameMnemonic(mnemonic, format, opcode string) (string, string) {

	var mn, enc string

	switch format {
	case "E":
		mn, enc = mnemonic_E(mnemonic, opcode)
	case "I":
		mn, enc = mnemonic_I(mnemonic, opcode)
	case "IE":
		mn, enc = mnemonic_IE(mnemonic, opcode)
	case "MII":
		mn, enc = mnemonic_MII(mnemonic, opcode)
	case "RI-a", "RI-b", "RI-c":
		mn, enc = mnemonic_RI(mnemonic, format, opcode)
	case "RIE-a", "RIE-b", "RIE-c", "RIE-d", "RIE-e", "RIE-f", "RIE-g":
		mn, enc = mnemonic_RIE(mnemonic, format, opcode)
	case "RIL-a", "RIL-b", "RIL-c":
		mn, enc = mnemonic_RIL(mnemonic, format, opcode)
	case "RIS":
		mn, enc = mnemonic_RIS(mnemonic, opcode)
	case "RR":
		mn, enc = mnemonic_RR(mnemonic, opcode)
	case "RRD":
		mn, enc = mnemonic_RRD(mnemonic, opcode)
	case "RRE":
		mn, enc = mnemonic_RRE(mnemonic, opcode)
	case "RRF-a", "RRF-b", "RRF-c", "RRF-d", "RRF-e":
		mn, enc = mnemonic_RRF(mnemonic, format, opcode)
	case "RRS":
		mn, enc = mnemonic_RRS(mnemonic, opcode)
	case "RS-a", "RS-b":
		mn, enc = mnemonic_RS(mnemonic, format, opcode)
	case "RSI":
		mn, enc = mnemonic_RSI(mnemonic, opcode)
	case "RSL-a", "RSL-b":
		mn, enc = mnemonic_RSL(mnemonic, format, opcode)
	case "RSY-a", "RSY-b":
		mn, enc = mnemonic_RSY(mnemonic, format, opcode)
	case "RX-a", "RX-b":
		mn, enc = mnemonic_RX(mnemonic, format, opcode)
	case "RXE":
		mn, enc = mnemonic_RXE(mnemonic, opcode)
	case "RXF":
		mn, enc = mnemonic_RXF(mnemonic, opcode)
	case "RXY-a", "RXY-b":
		mn, enc = mnemonic_RXY(mnemonic, format, opcode)
	case "S":
		mn, enc = mnemonic_S(mnemonic, opcode)
	case "SI":
		mn, enc = mnemonic_SI(mnemonic, opcode)
	case "SIL":
		mn, enc = mnemonic_SIL(mnemonic, opcode)
	case "SIY":
		mn, enc = mnemonic_SIY(mnemonic, opcode)
	case "SMI":
		mn, enc = mnemonic_SMI(mnemonic, opcode)
	case "SS-a", "SS-b", "SS-c", "SS-d", "SS-e", "SS-f":
		mn, enc = mnemonic_SS(mnemonic, format, opcode)
	case "SSE":
		mn, enc = mnemonic_SSE(mnemonic, opcode)
	case "SSF":
		mn, enc = mnemonic_SSF(mnemonic, opcode)
	case "VRI-a", "VRI-b", "VRI-c", "VRI-d", "VRI-e", "VRI-f", "VRI-g", "VRI-h", "VRI-i":
		mn, enc = mnemonic_VRI(mnemonic, format, opcode)
	case "VRR-a", "VRR-b", "VRR-c", "VRR-d", "VRR-e", "VRR-f", "VRR-g", "VRR-h", "VRR-i", "VRR-j", "VRR-k":
		mn, enc = mnemonic_VRR(mnemonic, format, opcode)
	case "VRS-a", "VRS-b", "VRS-c", "VRS-d":
		mn, enc = mnemonic_VRS(mnemonic, format, opcode)
	case "VRV":
		mn, enc = mnemonic_VRV(mnemonic, opcode)
	case "VRX":
		mn, enc = mnemonic_VRX(mnemonic, opcode)
	case "VSI":
		mn, enc = mnemonic_VSI(mnemonic, opcode)
	default:
		mn = mnemonic
	}
	return mn, enc
}

func mnemonic_E(mnemonic, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|??@16"
	return mnemonic, enc
}

func mnemonic_I(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " I"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|I@8|??@16"
	return mnemonic, enc
}

func mnemonic_IE(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " I1,I2"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|//@16|I1@24|I2@28|??@32"
	return mnemonic, enc
}

func mnemonic_MII(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " M1,RI2,RI3"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|M1@8|RI2@12|RI3@24|??@48"
	return mnemonic, enc
}

func mnemonic_RI(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:3], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RI-a":
		mnemonic += " R1,I2"
		enc = str1 + "@0|R1@8|" + str2 + "@12|I2@16|??@32"
	case "RI-b":
		mnemonic += " R1,RI2"
		enc = str1 + "@0|R1@8|" + str2 + "@12|RI2@16|??@32"
	case "RI-c":
		mnemonic += " M1,RI2"
		enc = str1 + "@0|M1@8|" + str2 + "@12|RI2@16|??@32"
	}
	return mnemonic, enc
}

func mnemonic_RIE(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RIE-a":
		mnemonic += " R1,I2,M3"
		enc = str1 + "@0|R1@8|//@12|I2@16|M3@32|//@36|" + str2 + "@40|??@48"
	case "RIE-b":
		mnemonic += " R1,R2,M3,RI4"
		enc = str1 + "@0|R1@8|R2@12|RI4@16|M3@32|//@36|" + str2 + "@40|??@48"
	case "RIE-c":
		mnemonic += " R1,I2,M3,RI4"
		enc = str1 + "@0|R1@8|M3@12|RI4@16|I2@32|" + str2 + "@40|??@48"
	case "RIE-d":
		mnemonic += " R1,R3,I2"
		enc = str1 + "@0|R1@8|R3@12|I2@16|//@32|" + str2 + "@40|??@48"
	case "RIE-e":
		mnemonic += " R1,R3,RI2"
		enc = str1 + "@0|R1@8|R3@12|RI2@16|//@32|" + str2 + "@40|??@48"
	case "RIE-f":
		mnemonic += " R1,R2,I3,I4,I5"
		enc = str1 + "@0|R1@8|R2@12|I3@16|I4@24|I5@32|" + str2 + "@40|??@48"
	case "RIE-g":
		mnemonic += " R1,I2,M3"
		enc = str1 + "@0|R1@8|M3@12|I2@16|//@32|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_RIL(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RIL-a":
		mnemonic += " R1,I2"
		enc = str1 + "@0|R1@8|" + str2 + "@12|I2@16|??@48"
	case "RIL-b":
		mnemonic += " R1,RI2"
		enc = str1 + "@0|R1@8|" + str2 + "@12|RI2@16|??@48"
	case "RIL-c":
		mnemonic += " M1,RI2"
		enc = str1 + "@0|M1@8|" + str2 + "@12|RI2@16|??@48"
	}
	return mnemonic, enc
}

func mnemonic_RIS(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	mnemonic += " R1,I2,M3,D4(B4)"
	enc = str1 + "@0|R1@8|M3@12|B4@16|D4@20|I2@32|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func mnemonic_RR(mnemonic, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch mnemonic {
	case "BCR":
		mnemonic += " M1,R2"
		enc = str + "@0|M1@8|R2@12|??@16"
	case "SPM":
		mnemonic += " R1"
		enc = str + "@0|R1@8|//@12|??@16"
	default:
		mnemonic += " R1,R2"
		enc = str + "@0|R1@8|R2@12|??@16"
	}
	return mnemonic, enc
}

func mnemonic_RRD(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " R1,R3,R2"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|R1@16|//@20|R3@24|R2@28|??@32"
	return mnemonic, enc
}

func mnemonic_RRE(mnemonic, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch mnemonic {
	case "LZER", "LZDR", "LZXR", "EFPC", "EPAR", "EPAIR", "ESEA", "ESAIR", "ESAR", "ETND", "IAC", "IPM", "MSTA", "PTF", "SFASR", "SFPC", "SSAR", "SSAIR":
		mnemonic += " R1"
		enc = str + "@0|//@16|R1@24|//@28|??@32"
	case "NNPA", "PALB", "PCC", "PCKMO":
		enc = str + "@0|//@16|??@32"
	default:
		mnemonic += " R1,R2"
		enc = str + "@0|//@16|R1@24|R2@28|??@32"
	}
	return mnemonic, enc
}

func mnemonic_RRF(mnemonic, format, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch format {
	case "RRF-a":
		switch mnemonic {
		case "SELR", "SELGR", "SELFHR", "IPTE", "AXTRA", "ADTRA",
			"DDTRA", "DXTRA", "MDTRA", "MXTRA", "SDTRA", "SXTRA":
			mnemonic += " R1,R2,R3,M4"
			enc = str + "@0|R3@16|M4@20|R1@24|R2@28|??@32"
		default:
			mnemonic += " R1,R2,R3"
			enc = str + "@0|R3@16|//@20|R1@24|R2@28|??@32"
		}
	case "RRF-b":
		switch mnemonic {
		case "CRDTE", "IDTE", "LPTEA", "RDP", "DIEBR", "DIDBR",
			"QADTR", "QAXTR", "RRDTR", "RRXTR":
			mnemonic += " R1,R3,R2,M4"
			enc = str + "@0|R3@16|M4@20|R1@24|R2@28|??@32"
		default:
			mnemonic += " R1,R3,R2"
			enc = str + "@0|R3@16|//@20|R1@24|R2@28|??@32"
		}
	case "RRF-c":
		mnemonic += " R1,R2,M3"
		enc = str + "@0|M3@16|//@20|R1@24|R2@28|??@32"
	case "RRF-d":
		mnemonic += " R1,R2,M4"
		enc = str + "@0|//@16|M4@20|R1@24|R2@28|??@32"
	case "RRF-e":
		switch mnemonic {
		case "CXFBRA", "CXFTR", "CDFBRA", "CDFTR", "CEFBRA", "CXGBRA", "CXGTRA", "CDGBRA", "CDGTRA", "CEGBRA", "CXLFBR", "CXLFTR", "CDLFBR", "CDLFTR", "CELFBR",
			"CXLGBR", "CXLGTR", "CDLGBR", "CDLGTR", "CELGBR", "CFXBRA", "CGXBRA", "CFXTR", "CGXTRA", "CFDBRA", "CGDBRA", "CFDTR", "CGDTRA", "CFEBRA", "CGEBRA",
			"CLFEBR", "CLFDBR", "CLFXBR", "CLGEBR", "CLGDBR", "CLGXBR", "CLFXTR", "CLFDTR", "CLGXTR", "CLGDTR", "FIEBRA", "FIDBRA", "FIXBRA", "FIDTR", "FIXTR",
			"LDXBRA", "LEDBRA", "LEXBRA", "LEDTR", "LDXTR":
			mnemonic += " R1,M3,R2,M4"
			enc = str + "@0|M3@16|M4@20|R1@24|R2@28|??@32"
		default:
			mnemonic += " R1,M3,R2"
			enc = str + "@0|M3@16|//@20|R1@24|R2@28|??@32"
		}
	}
	return mnemonic, enc
}

func mnemonic_RRS(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " R1,R2,M3,D4(B4)"
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	enc = str1 + "@0|R1@8|R2@12|B4@16|D4@20|M3@32|//@36|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func mnemonic_RS(mnemonic, format, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch format {
	case "RS-a":
		switch mnemonic {
		case "SLDA", "SLDL", "SLA", "SLL", "SRA", "SRDA", "SRDL", "SRL":
			mnemonic += " R1,D2(B2)"
			enc = str + "@0|R1@8|//@12|B2@16|D2@20|??@32"
		default:
			mnemonic += " R1,R3,D2(B2)"
			enc = str + "@0|R1@8|R3@12|B2@16|D2@20|??@32"
		}
	case "RS-b":
		mnemonic += " R1,M3,D2(B2)"
		enc = str + "@0|R1@8|M3@12|B2@16|D2@20|??@32"
	}
	return mnemonic, enc
}

func mnemonic_RSI(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " R1,R3,RI2"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|R1@8|R3@12|RI2@16|??@32"
	return mnemonic, enc
}

func mnemonic_RSL(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RSL-a":
		mnemonic += " D1(L1,B1)"
		enc = str1 + "@0|L1@8|//@12|B1@16|D1@20|//@32|" + str2 + "@40|??@48"
	case "RSL-b":
		mnemonic += " R1,D2(L2,B2),M3"
		enc = str1 + "@0|L2@8|B2@16|D2@20|R1@32|M3@36|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_RSY(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RSY-a":
		mnemonic += " R1,R3,D2(B2)"
		enc = str1 + "@0|R1@8|R3@12|B2@16|D2@20|" + str2 + "@40|??@48"
	case "RSY-b":
		switch mnemonic {
		case "LOC", "LOCFH", "LOCG", "STOCFH", "STOC", "STOCG":
			mnemonic += " R1,D2(B2),M3"
		default:
			mnemonic += " R1,M3,D2(B2)"
		}
		enc = str1 + "@0|R1@8|M3@12|B2@16|D2@20|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_RX(mnemonic, format, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseInt(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch format {
	case "RX-a":
		mnemonic += " R1,D2(X2,B2)"
		enc = str + "@0|R1@8|X2@12|B2@16|D2@20|??@32"
	case "RX-b":
		mnemonic += " M1,D2(X2,B2)"
		enc = str + "@0|M1@8|X2@12|B2@16|D2@20|??@32"
	}
	return mnemonic, enc
}

func mnemonic_RXE(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch mnemonic {
	case "LCBB":
		mnemonic += " R1,D2(X2,B2),M3"
		enc = str1 + "@0|R1@8|X2@12|B2@16|D2@20|M3@32|//@36|" + str2 + "@40|??@48"
	default:
		mnemonic += " R1,D2(X2,B2)"
		enc = str1 + "@0|R1@8|X2@12|B2@16|D2@20|//@32|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_RXF(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " R1,R3,D2(X2,B2)"
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	enc = str1 + "@0|R3@8|X2@12|B2@16|D2@20|R1@32|//@36|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func mnemonic_RXY(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "RXY-a":
		mnemonic += " R1,D2(X2,B2)"
		enc = str1 + "@0|R1@8|X2@12|B2@16|D2@20|" + str2 + "@40|??@48"
	case "RXY-b":
		mnemonic += " M1,D2(X2,B2)"
		enc = str1 + "@0|M1@8|X2@12|B2@16|D2@20|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_S(mnemonic, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch mnemonic {
	case "PTLB", "TEND", "XSCH", "CSCH", "HSCH", "IPK", "RCHP", "RSCH", "SAL", "SCHM":
		enc = str + "@0|//@16|??@32"
	default:
		mnemonic += " D2(B2)"
		enc = str + "@0|B2@16|D2@20|??@32"
	}
	return mnemonic, enc
}

func mnemonic_SI(mnemonic, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch mnemonic {
	case "TS", "SSM", "LPSW":
		mnemonic += " D1(B1)"
	default:
		mnemonic += " D1(B1),I2"
	}
	enc = str + "@0|I2@8|B1@16|D1@20|??@32"
	return mnemonic, enc
}

func mnemonic_SIL(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " D1(B1),I2"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|B1@16|D1@20|I2@32|??@48"
	return mnemonic, enc
}

func mnemonic_SIY(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch mnemonic {
	case "LPSWEY":
		mnemonic += " D1(B1)"
		enc = str1 + "@0|//@8|B1@16|D1@20|" + str2 + "@40|??@48"
	default:
		mnemonic += " D1(B1),I2"
		enc = str1 + "@0|I2@8|B1@16|D1@20|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_SMI(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " M1,RI2,D3(B3)"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|M1@8|//@12|B3@16|D3@20|RI2@32|??@48"
	return mnemonic, enc
}

func mnemonic_SS(mnemonic, format, opcode string) (string, string) {
	var enc string
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	switch format {
	case "SS-a":
		mnemonic += " D1(L1,B1),D2(B2)"
		enc = str + "@0|L1@8|B1@16|D1@20|B2@32|D2@36|??@48"
	case "SS-b":
		mnemonic += " D1(L1,B1),D2(L2,B2)"
		enc = str + "@0|L1@8|L2@12|B1@16|D1@20|B2@32|D2@36|??@48"
	case "SS-c":
		mnemonic += " D1(L1,B1),D2(B2),I3"
		enc = str + "@0|L1@8|I3@12|B1@16|D1@20|B2@32|D2@36|??@48"
	case "SS-d":
		mnemonic += " D1(R1,B1),D2(B2),R3"
		enc = str + "@0|R1@8|R3@12|B1@16|D1@20|B2@32|D2@36|??@48"
	case "SS-e":
		switch mnemonic {
		case "LMD":
			mnemonic += " R1,R3,D2(B2),D4(B4)"
		default:
			mnemonic += " R1,D2(B2),R3,D4(B4)"
		}
		enc = str + "@0|R1@8|R3@12|B2@16|D2@20|B4@32|D4@36|??@48"
	case "SS-f":
		mnemonic += " D1(B1),D2(L2,B2)"
		enc = str + "@0|L2@8|B1@16|D1@20|B2@32|D2@36|??@48"
	}
	return mnemonic, enc

}

func mnemonic_SSE(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " D1(B1),D2(B2)"
	val, _ := strconv.ParseUint(opcode, 16, 16)
	str := strconv.Itoa(int(val))
	enc = str + "@0|B1@16|D1@20|B2@32|D2@36|??@48"
	return mnemonic, enc
}

func mnemonic_SSF(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch mnemonic {
	case "LPD", "LPDG":
		mnemonic += " R3,D1(B1),D2(B2)"
	default:
		mnemonic += " D1(B1),D2(B2),R3"
	}
	enc = str1 + "@0|R3@8|" + str2 + "@12|B1@16|D1@20|B2@32|D2@36|??@48"
	return mnemonic, enc
}

func mnemonic_VRI(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "VRI-a":
		if strings.Contains(mnemonic, "VGBM") { // Check for M3 field
			mnemonic += " V1,I2"
			enc = str1 + "@0|V1@8|//@12|I2@16|//@32|RXB@36|" + str2 + "@40|??@48"
		} else {
			mnemonic += " V1,I2,M3"
			enc = str1 + "@0|V1@8|//@12|I2@16|M3@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRI-b":
		mnemonic += " V1,I2,I3,M4"
		enc = str1 + "@0|V1@8|//@12|I2@16|I3@24|M4@32|RXB@36|" + str2 + "@40|??@48"
	case "VRI-c":
		mnemonic += " V1,V3,I2,M4"
		enc = str1 + "@0|V1@8|V3@12|I2@16|M4@32|RXB@36|" + str2 + "@40|??@48"
	case "VRI-d":
		if strings.Contains(mnemonic, "VERIM") { // Check for M5 field
			mnemonic += " V1,V2,V3,I4,M5"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|I4@24|M5@32|RXB@36|" + str2 + "@40|??@48"
		} else {
			mnemonic += " V1,V2,V3,I4"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|I4@24|//@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRI-e":
		mnemonic += " V1,V2,I3,M4,M5"
		enc = str1 + "@0|V1@8|V2@12|I3@16|M5@28|M4@32|RXB@36|" + str2 + "@40|??@48"
	case "VRI-f":
		mnemonic += " V1,V2,V3,I4,M5"
		enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M5@24|I4@28|RXB@36|" + str2 + "@40|??@48"
	case "VRI-g":
		mnemonic += " V1,V2,I3,I4,M5"
		enc = str1 + "@0|V1@8|V2@12|I4@16|M5@24|I3@28|RXB@36|" + str2 + "@40|??@48"
	case "VRI-h":
		mnemonic += " V1,I2,I3"
		enc = str1 + "@0|V1@8|//@12|I2@16|I3@32|RXB@36|" + str2 + "@40|??@48"
	case "VRI-i":
		mnemonic += " V1,R2,I3,M4"
		enc = str1 + "@0|V1@8|R2@12|//@16|M4@24|I3@28|RXB@36|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_VRR(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "VRR-a":
		switch mnemonic {
		case "VLR", "VTM": // V1,V2
			mnemonic += " V1,V2"
			enc = str1 + "@0|V1@8|V2@12|//@16|RXB@36|" + str2 + "@40|??@48"

		case "VSEG", "VUPH", "VUPLH", "VUPL", "VUPLL", "VCLZ", "VCTZ", "VEC", "VECL", "VLC", "VLP", "VPOPCT": // V1,V2,M3
			mnemonic += " V1,V2,M3"
			enc = str1 + "@0|V1@8|V2@12|//@16|M3@32|RXB@36|" + str2 + "@40|??@48"

		case "VISTR": // V1,V2,M3,M5
			mnemonic += " V1,V2,M3,M5"
			enc = str1 + "@0|V1@8|V2@12|//@16|M5@24|//@28|M3@32|RXB@36|" + str2 + "@40|??@48"

		case "WFC", "WFK", "VFLL", "VFSQ", "VCLFNH", "VCLFNL", "VCFN", "VCNF": // V1,V2,M3,M4
			mnemonic += " V1,V2,M3,M4"
			enc = str1 + "@0|V1@8|V2@12|//@16|M4@28|M3@32|RXB@36|" + str2 + "@40|??@48"

		case "VCFPS", "VCDG", "VCDLG", "VCGD", "VCFPL", "VCSFP", "VCLFP", "VCLGD", "VFI", "VFLR", "VFPSO": // V1,V2,M3,M4,M5
			mnemonic += " V1,V2,M3,M4,M5"
			enc = str1 + "@0|V1@8|V2@12|//@16|M5@24|M4@28|M3@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRR-b":
		switch mnemonic {
		case "VSCSHP":
			mnemonic += " V1,V2,V3"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|RXB@36|" + str2 + "@40|??@48"
		default:
			mnemonic += " V1,V2,V3,M4,M5"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M5@24|//@28|M4@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRR-c":
		switch mnemonic {
		case "VFA", "VFD", "VFM", "VFS", "VCRNF": // V1,V2,V3,M4,M5
			mnemonic += " V1,V2,V3,M4,M5"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M5@28|M4@32|RXB@36|" + str2 + "@40|??@48"

		case "VFCE", "VFCH", "VFCHE", "VFMAX", "VFMIN": // V1,V2,V3,M4,M5,M6
			mnemonic += " V1,V2,V3,M4,M5,M6"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M6@24|M5@28|M4@32|RXB@36|" + str2 + "@40|??@48"

		case "VBPERM", "VN", "VNC", "VCKSM", "VX", "VNN", "VNO", "VNX",
			"VO", "VOC", "VSL", "VSLB", "VSRA", "VSRAB", "VSRL", "VSRLB": // V1,V2,V3
			mnemonic += " V1,V2,V3"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|RXB@36|" + str2 + "@40|??@48"
		default: // V1,V2,V3,M4
			mnemonic += " V1,V2,V3,M4"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M4@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRR-d":
		switch mnemonic {
		case "VMSL", "VSTRC", "VSTRS": // V1,V2,V3,V4,M5,M6
			mnemonic += " V1,V2,V3,V4,M5,M6"
			enc = str1 + "@0|V1@8|V2@12|V3@16|M5@20|M6@24|//@28|V4@32|RXB@36|" + str2 + "@40|??@48"
		default:
			mnemonic += " V1,V2,V3,V4,M5"
			enc = str1 + "@0|V1@8|V2@12|V3@16|M5@20|//@24|V4@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRR-e":
		switch mnemonic {
		case "VPERM", "VSEL":
			mnemonic += " V1,V2,V3,V4"
			enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|V4@32|RXB@36|" + str2 + "@40|??@48"
		default:
			mnemonic += " V1,V2,V3,V4,M5,M6"
			enc = str1 + "@0|V1@8|V2@12|V3@16|M6@20|//@24|M5@28|V4@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRR-f":
		mnemonic += " V1,R2,R3"
		enc = str1 + "@0|V1@8|R2@12|R3@16|//@20|RXB@36|" + str2 + "@40|??@48"
	case "VRR-g":
		mnemonic += " V1"
		enc = str1 + "@0|//@8|V1@12|//@16|RXB@36|" + str2 + "@40|??@48"
	case "VRR-h":
		mnemonic += " V1,V2,M3"
		enc = str1 + "@0|//@8|V1@12|V2@16|//@20|M3@24|//@28|RXB@36|" + str2 + "@40|??@48"
	case "VRR-i":
		mnemonic += " R1,V2,M3,M4"
		enc = str1 + "@0|R1@8|V2@12|//@16|M3@24|M4@28|//@32|RXB@36|" + str2 + "@40|??@48"
	case "VRR-j":
		mnemonic += " V1,V2,V3,M4"
		enc = str1 + "@0|V1@8|V2@12|V3@16|//@20|M4@24|//@28|RXB@36|" + str2 + "@40|??@48"
	case "VRR-k":
		mnemonic += " V1,V2,M3"
		enc = str1 + "@0|V1@8|V2@12|//@16|M3@24|//@28|RXB@36|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_VRS(mnemonic, format, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	switch format {
	case "VRS-a":
		mnemonic += " V1,V3,D2(B2),M4"
		enc = str1 + "@0|V1@8|V3@12|B2@16|D2@20|M4@32|RXB@36|" + str2 + "@40|??@48"
	case "VRS-b":
		if strings.Contains(mnemonic, "VLVG") {
			mnemonic += " V1,R3,D2(B2),M4"
			enc = str1 + "@0|V1@8|R3@12|B2@16|D2@20|M4@32|RXB@36|" + str2 + "@40|??@48"
		} else {
			mnemonic += " V1,R3,D2(B2)"
			enc = str1 + "@0|V1@8|R3@12|B2@16|D2@20|//@32|RXB@36|" + str2 + "@40|??@48"
		}
	case "VRS-c":
		mnemonic += " R1,V3,D2(B2),M4"
		enc = str1 + "@0|R1@8|V3@12|B2@16|D2@20|M4@32|RXB@36|" + str2 + "@40|??@48"
	case "VRS-d":
		mnemonic += " V1,R3,D2(B2)"
		enc = str1 + "@0|//@8|R3@12|B2@16|D2@20|V1@32|RXB@36|" + str2 + "@40|??@48"
	}
	return mnemonic, enc
}

func mnemonic_VRV(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	mnemonic += " V1,D2(V2,B2),M3"
	enc = str1 + "@0|V1@8|V2@12|B2@16|D2@20|M3@32|RXB@36|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func mnemonic_VRX(mnemonic, opcode string) (string, string) {
	var enc string
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	mnemonic += " V1,D2(X2,B2),M3"
	enc = str1 + "@0|V1@8|X2@12|B2@16|D2@20|M3@32|RXB@36|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func mnemonic_VSI(mnemonic, opcode string) (string, string) {
	var enc string
	mnemonic += " V1,D2(B2),I3"
	val1, _ := strconv.ParseUint(opcode[:2], 16, 16)
	str1 := strconv.Itoa(int(val1))
	val2, _ := strconv.ParseUint(opcode[2:], 16, 16)
	str2 := strconv.Itoa(int(val2))
	enc = str1 + "@0|I3@8|B2@16|D2@20|V1@32|RXB@36|" + str2 + "@40|??@48"
	return mnemonic, enc
}

func matchXCord(t pdf.Text, Xcord float64) bool {
	return math.Abs(t.X-Xcord) < 0.9
}

func match(t pdf.Text, font string, size float64, substr string) bool {
	return t.Font == font && (size == 0 || math.Abs(t.FontSize-size) < 0.2) && strings.Contains(t.S, substr)
}

func findWords(chars []pdf.Text) (words []pdf.Text) {
	// Sort by Y coordinate and normalize.
	const nudge = 1.5
	sort.Sort(pdf.TextVertical(chars))
	old := -100000.0
	for i, c := range chars {
		if c.Y != old && math.Abs(old-c.Y) < nudge {
			chars[i].Y = old
		} else {
			old = c.Y
		}
	}

	// Sort by Y coordinate, breaking ties with X.
	// This will bring letters in a single word together.
	sort.Sort(pdf.TextVertical(chars))

	// Loop over chars.
	for i := 0; i < len(chars); {
		// Find all chars on line.
		j := i + 1
		for j < len(chars) && chars[j].Y == chars[i].Y {
			j++
		}
		var end float64
		// Split line into words (really, phrases).
		for k := i; k < j; {
			ck := &chars[k]
			s := ck.S
			end = ck.X + ck.W
			charSpace := ck.FontSize / 6
			wordSpace := ck.FontSize * 2 / 3
			l := k + 1
			for l < j {
				// Grow word.
				cl := &chars[l]
				if sameFont(cl.Font, ck.Font) && math.Abs(cl.FontSize-ck.FontSize) < 0.1 && cl.X <= end+charSpace {
					s += cl.S
					end = cl.X + cl.W
					l++
					continue
				}
				// Add space to phrase before next word.
				if sameFont(cl.Font, ck.Font) && math.Abs(cl.FontSize-ck.FontSize) < 0.1 && cl.X <= end+wordSpace {
					s += " " + cl.S
					end = cl.X + cl.W
					l++
					continue
				}
				break
			}
			f := ck.Font
			f = strings.TrimSuffix(f, ",Italic")
			f = strings.TrimSuffix(f, "-Italic")
			words = append(words, pdf.Text{f, ck.FontSize, ck.X, ck.Y, end - ck.X, s})
			k = l
		}
		i = j
	}
	return words
}

func sameFont(f1, f2 string) bool {
	f1 = strings.TrimSuffix(f1, ",Italic")
	f1 = strings.TrimSuffix(f1, "-Italic")
	f2 = strings.TrimSuffix(f1, ",Italic")
	f2 = strings.TrimSuffix(f1, "-Italic")
	return strings.TrimSuffix(f1, ",Italic") == strings.TrimSuffix(f2, ",Italic") || f1 == "Symbol" || f2 == "Symbol" || f1 == "TimesNewRoman" || f2 == "TimesNewRoman"
}
