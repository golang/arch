// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// loong64spec reads the "LoongArchReferenceManual-VolumeII(VectorInstructionExtensions)V1.11.pdf" [1]
// to collect instruction encoding details and output to tables.go.
//
// usage: go run specSIMD.go LoongArchReferenceManual-VolumeII(VectorInstructionExtensions)V1.11.pdf
//
// [1]: https://file.loongfans.cn/loonginfo/LoongArchReferenceManual-VolumeII%28VectorInstructionExtensions%29V1.11.pdf
//
// Note: Before running this tool, please first execute the loong64spec/spec.go
// tool to generate the base instruction table file. This tool focuses on
// appending new SIMD instructions to the `tables.go` file.

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

func mergeMap(m1 map[string]string, m2 map[string]string) {
	for k := range m2 {
		m1[k] = m2[k]
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("loong64specSIMD: ")

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: loong64specSIMD LoongArchReferenceManual-VolumeII(VectorInstructionExtensions)V1.11.pdf\n")
		os.Exit(2)
	}
	content, err := os.ReadFile("../tables.go")
	if err != nil {
		e := fmt.Sprintf("%s", err.Error())
		if strings.Contains(e, "no such file or directory") {
			log.Fatal("The file 'tables.go' does not exist, please generate base instructions table using the tool loong64spec/base/spec.go.\n")
		}
		log.Fatal(err)
	}
	tables := string(content)
	strs := strings.Split(tables, ")")
	curOpCon := strs[0]
	strs = strings.Split(strs[1], "var instFormats = [...]instFormat{")
	curOpstrs := strings.Split("var opstr = [...]string{"+strings.Split(strs[0], "var opstr = [...]string{")[1], "}")[0]
	curInstFormats := "var instFormats = [...]instFormat{" + strings.Split(strs[1], ",\n}")[0] + ",\n"

	var op_f bytes.Buffer
	op_f.Write([]byte(curOpCon))

	var opstr_f bytes.Buffer
	opstr_f.Write([]byte(curOpstrs))

	var instFormats_f bytes.Buffer
	instFormats_f.Write([]byte(curInstFormats))

	f, r, err := pdf.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Scan document looking for instructions.
	n := r.NumPage()
	var ops []string
	opstrs := map[string]string{}
	instFormatComments := map[string]string{}
	instFormats := map[string]string{}
	var fp int
	for pageNum := 1; pageNum <= n; pageNum++ {
		p := r.Page(pageNum)
		if fp == 0 {
			if !isFirstPage(p) {
				continue
			}
			fp = pageNum
		}
		cPageOps, cPageOpstrs, cPageInstFormatComments, cPageInstFormats := parsePage(pageNum, p, fp == pageNum)
		ops = append(ops, cPageOps...)
		mergeMap(opstrs, cPageOpstrs)
		mergeMap(instFormatComments, cPageInstFormatComments)
		mergeMap(instFormats, cPageInstFormats)
	}

	sort.Strings(ops)

	for _, op := range ops {
		// 1. write op
		op_f.Write([]byte(fmt.Sprintf("\t%s\n", op)))
		// 2. write opstr
		opstr_f.Write([]byte(fmt.Sprintf("\t%s\n", opstrs[op])))
		// 3. write instFormat
		instFormats_f.Write([]byte(fmt.Sprintf("\t%s\n\t%s\n", instFormatComments[op], instFormats[op])))
	}

	op_f.Write([]byte(")\n\n"))
	opstr_f.Write([]byte("}\n\n"))
	instFormats_f.Write([]byte("}\n"))

	fileTables, _ := os.Create("../tables.go")
	defer fileTables.Close()

	fileTables.Write(op_f.Bytes())
	fileTables.Write(opstr_f.Bytes())
	fileTables.Write(instFormats_f.Bytes())
}

func isFirstPage(page pdf.Page) bool {
	content := page.Content()
	simdAppendixbPrefix := "LoongArchReferenceManualVolumeII-a:VectorInstructionExtensions"
	simdAppendixbSuffix := "AppendixB"
	simdAppendixbLen := len(simdAppendixbPrefix) + len(simdAppendixbSuffix) + 4
	ct := ""
	for _, t := range content.Text {
		ct += t.S
		if strings.HasSuffix(ct, simdAppendixbSuffix) {
			return true
		}
		if strings.HasPrefix(simdAppendixbPrefix, ct) || strings.HasPrefix(ct, simdAppendixbPrefix) {
			if len(ct) >= simdAppendixbLen {
				return false
			}
			continue
		} else {
			return false

		}
	}
	return false
}

func getArg(opName string, name string) (length int, argName string) {
	switch {
	case strings.Contains("arg_va", name):
		return 5, "arg_va"
	case strings.Contains("arg_vk", name):
		return 5, "arg_vk"
	case strings.Contains("arg_vj", name):
		return 5, "arg_vj"
	case strings.Contains("arg_vd", name):
		return 5, "arg_vd"

	case strings.Contains("arg_xa", name):
		return 5, "arg_xa"
	case strings.Contains("arg_xk", name):
		return 5, "arg_xk"
	case strings.Contains("arg_xj", name):
		return 5, "arg_xj"
	case strings.Contains("arg_xd", name) || strings.Contains("arg_XD", name):
		return 5, "arg_xd"

	case strings.Contains("arg_rd", name):
		return 5, "arg_rd"
	case strings.Contains("arg_rj", name):
		return 5, "arg_rj"
	case strings.Contains("arg_rk", name):
		return 5, "arg_rk"

	case strings.Contains("arg_cd", name):
		return 3, "arg_cd"

	case name == "idx":
		switch opName {
		case "VSTELM_W", "XVSTELM_D":
			return 2, "arg_idx2_19_18"
		case "VSTELM_H", "XVSTELM_W":
			return 3, "arg_idx3_20_18"
		case "VSTELM_B", "XVSTELM_H":
			return 4, "arg_idx4_21_18"
		case "XVSTELM_B":
			return 5, "arg_idx5_22_18"
		}
		log.Fatal("unprocessed argument: ", name)

	case strings.Contains(name, "ui"):
		length, _ = strconv.Atoi(strings.Split(name, "ui")[1])
		switch length {
		default:
			log.Fatal("unprocessed argument: ", name)
		case 2:
			argName = "arg_ui2_11_10"
		case 3:
			argName = "arg_ui3_12_10"
		case 4:
			argName = "arg_ui4_13_10"
		case 5:
			argName = "arg_ui5_14_10"
		case 6:
			argName = "arg_ui6_15_10"
		case 7:
			argName = "arg_ui7_16_10"
		case 8:
			argName = "arg_ui8_17_10"
		}

	case strings.Contains(name, "si"):
		length, _ = strconv.Atoi(strings.Split(name, "si")[1])
		switch length {
		default:
			log.Fatal("unprocessed argument: ", name)
		case 5:
			argName = "arg_si5_14_10"
		case 8:
			argName = "arg_si8_17_10"
		case 9:
			argName = "arg_si9_18_10"
		case 10:
			argName = "arg_si10_19_10"
		case 11:
			argName = "arg_si11_20_10"
		case 12:
			argName = "arg_si12_21_10"
		}

	case strings.HasPrefix(name, "i"):
		length, _ = strconv.Atoi(strings.Split(name, "i")[1])
		switch length {
		default:
			log.Fatal("unprocessed argument: ", name)
		case 13:
			argName = "arg_i13_17_5"
		}
	}

	return
}

func binstrToHex(str string) string {
	rst := 0
	hex := "0x"
	charArray := []byte(str)
	for i := 0; i < 32; {
		rst = 1*(int(charArray[i+3])-48) + 2*(int(charArray[i+2])-48) + 4*(int(charArray[i+1])-48) + 8*(int(charArray[i])-48)
		switch rst {
		case 10:
			hex = hex + "a"
		case 11:
			hex = hex + "b"
		case 12:
			hex = hex + "c"
		case 13:
			hex = hex + "d"
		case 14:
			hex = hex + "e"
		case 15:
			hex = hex + "f"
		default:
			hex += strconv.Itoa(rst)
		}

		i = i + 4
	}
	return hex
}

/*
Here we deal with the instruction FCMP.cond.S/D, which has the following format:

	| 31 - 20 | 19 - 15 | 14 - 10 | 9 - 5 | 4 | 3 | 2 - 0 |
	|---------|---------|---------|-------|---|---|-------|
	|   op    |  cond   |    fk   |   fj  | 0 | 0 |  cd   |

The `cond` field has these possible values:

	"CAF": "00",
	"CUN": "08",
	"CEQ": "04",
	"CUEQ": "0c",
	"CLT": "02",
	"CULT": "0a",
	"CLE": "06",
	"CULE": "0e",
	"CNE": "10",
	"COR": "14",
	"CUNE": "18",
	"SAF": "01",
	"SUN": "09",
	"SEQ": "05",
	"SUEQ": "0d",
	"SLT": "03",
	"SULT": "0b",
	"SLE": "07",
	"SULE": "0f",
	"SNE": "11",
	"SOR": "15",
	"SUNE": "19",

These values are the hexadecimal numbers of bits 19 to 15, the same as
described in the instruction set manual.

The following code defines a map, the values in it represent the hexadecimal
encoding of the cond field in the entire instruction. In this case, the upper
4 bits and the lowest 1 bit are encoded separately, so the encoding is
different from the encoding described above.
*/
func dealWithFcmp(instName string) (fcmpConditions map[string]map[string]string) {
	conds := map[string]string{
		"CAF":  "00",
		"CUN":  "40",
		"CEQ":  "20",
		"CUEQ": "60",
		"CLT":  "10",
		"CULT": "50",
		"CLE":  "30",
		"CULE": "70",
		"CNE":  "80",
		"COR":  "a0",
		"CUNE": "c0",
		"SAF":  "08",
		"SUN":  "48",
		"SEQ":  "28",
		"SUEQ": "68",
		"SLT":  "18",
		"SULT": "58",
		"SLE":  "38",
		"SULE": "78",
		"SNE":  "88",
		"SOR":  "a8",
		"SUNE": "c8",
	}

	insP := strings.Split(instName, ".") // ([X]VFCMP, cond, S/D)
	prefix := insP[0]
	ds := insP[2]
	fcmpConditions = make(map[string]map[string]string)
	argStr := "arg_vd, arg_vj, arg_vk"
	value := "0x0c5" // VFCMP, S
	switch {
	case insP[0] == "VFCMP" && insP[2] == "S":
	case insP[0] == "VFCMP" && insP[2] == "D":
		value = "0x0c6"
	case insP[0] == "XVFCMP" && insP[2] == "S":
		value = "0x0c9"
		argStr = "arg_xd, arg_xj, arg_xk"
	case insP[0] == "XVFCMP" && insP[2] == "D":
		value = "0x0ca"
		argStr = "arg_xd, arg_xj, arg_xk"
	}
	for k, v := range conds {
		op := fmt.Sprintf("%s_%s_%s", prefix, k, ds)
		opstr := fmt.Sprintf("%s_%s_%s:\t\"%s.%s.%s\",", prefix, k, ds, prefix, k, ds)
		instFormatComment := fmt.Sprintf("// %s.%s.%s %s", prefix, k, ds, argStr)
		instFormat := fmt.Sprintf("{mask: 0xffff8000, value: %s%s000, op: %s_%s_%s, args: instArgs{%s}},", value, v, prefix, k, ds, argStr)

		fcmpConditions[op] = make(map[string]string)
		fcmpConditions[op]["op"] = op
		fcmpConditions[op]["opstr"] = opstr
		fcmpConditions[op]["instFormatComment"] = instFormatComment
		fcmpConditions[op]["instFormat"] = instFormat
	}
	return
}

func needSpecialHandle(op string) bool {
	switch op {
	case "XVBITSEL_V", "VSHUF_B", "VSTELM_D", "VINSGR2VR_D", "VPICKVE2GR_D", "VPICKVE2GR_DU", "VREPLVEI_D", "XVREPL128VEI_D":
		return true
	}
	return false
}

func dealWithSpecialInst(op string) (comment string, instFormat string, nextI int) {
	switch op {
	case "XVBITSEL_V":
		comment = "// XVBITSEL.V xd, xj, xk, xa"
		instFormat = fmt.Sprintf("{mask: 0xfff00000, value: 0x0d200000, op: %s, args: instArgs{arg_xd, arg_xj, arg_xk, arg_xa}},", op)
		nextI = 10
	case "VSHUF_B":
		comment = "// VSHUF.B vd, vj, vk, va"
		instFormat = fmt.Sprintf("{mask: 0xfff00000, value: 0x0d500000, op: %s, args: instArgs{arg_vd, arg_vj, arg_vk, arg_va}},", op)
		nextI = 4
	case "VSTELM_D":
		comment = "// VSTELM.D vd, rj, si8, idx"
		instFormat = fmt.Sprintf("{mask: 0xfff00000, value: 0x31100000, op: %s, args: instArgs{arg_vd, arg_rj, arg_si8_17_10, arg_idx1_18_18}},", op)
		nextI = 8

	case "VINSGR2VR_D":
		comment = "// VINSGR2VR.D vd, rj, ui1"
		instFormat = fmt.Sprintf("{mask: 0xfffff000, value: 0x72ebf000, op: %s, args: instArgs{arg_vd, arg_rj, arg_ui1_10_10}},", op)
		nextI = 7

	case "VPICKVE2GR_D":
		comment = "// VPICKVE2GR.D rd, vj, ui1"
		instFormat = fmt.Sprintf("{mask: 0xfffff000, value: 0x72eff000, op: %s, args: instArgs{arg_rd, arg_vj, arg_ui1_10_10}},", op)
		nextI = 7

	case "VPICKVE2GR_DU":
		comment = "// VPICKVE2GR.DU rd, vj, ui1"
		instFormat = fmt.Sprintf("{mask: 0xfffff000, value: 0x72f3f000, op: %s, args: instArgs{arg_rd, arg_vj, arg_ui1_10_10}},", op)
		nextI = 7

	case "VREPLVEI_D":
		comment = "// VREPLVEI.D vd, vj, ui1"
		instFormat = fmt.Sprintf("{mask: 0xfffff000, value: 0x72f7f000, op: %s, args: instArgs{arg_vd, arg_vj, arg_ui1_10_10}},", op)
		nextI = 7

	case "XVREPL128VEI_D":
		comment = "// XVREPL128VEI.D xd, xj, ui1"
		instFormat = fmt.Sprintf("{mask: 0xfffff000, value: 0x76f7f000, op: %s, args: instArgs{arg_xd, arg_xj, arg_ui1_10_10}},", op)
		nextI = 7
	}
	return
}

func findWords(chars []pdf.Text) (words []pdf.Text) {
	for i := 0; i < len(chars); {
		xRange := []float64{chars[i].X, chars[i].X}
		j := i + 1

		// Find all chars on one line.
		for j < len(chars) && chars[j].Y == chars[i].Y {
			xRange[1] = chars[j].X
			j++
		}

		// Split line into words (really, phrases).
		for k := i; k < j; {
			ck := &chars[k]
			s := ck.S
			var preX float64 = ck.X
			l := k + 1
			for ; l < j; l++ {
				cl := &chars[l]
				// Based on observation, if the distance between two characters exceeds 10,
				// they are considered not to belong to the same word.
				if cl.X-preX < 10 {
					s += cl.S
					preX = cl.X
					continue
				}
				if cl.X-preX >= 10 {
					f := ck.Font
					words = append(words, pdf.Text{
						Font:     f,
						FontSize: ck.FontSize,
						X:        ck.X,
						Y:        ck.Y,
						W:        preX - ck.X,
						S:        s,
					})
					s = cl.S
					preX = cl.X
					continue
				}
			}
			f := ck.Font
			words = append(words, pdf.Text{
				Font:     f,
				FontSize: ck.FontSize,
				X:        ck.X,
				Y:        ck.Y,
				W:        preX - ck.X,
				S:        s,
			})
			k = l
		}
		i = j
	}

	return words
}

func parsePage(num int, p pdf.Page, isFP bool) (ops []string, opstrs map[string]string, instFormatComments map[string]string, instFormats map[string]string) {
	opstrs = make(map[string]string)
	instFormatComments = make(map[string]string)
	instFormats = make(map[string]string)

	content := p.Content()

	var text []pdf.Text
	for _, t := range content.Text {
		text = append(text, t)
	}

	// page header(62), page num(3), table name(30), table header(64)
	if isFP {
		text = text[159:len(text)] // 62+3+30+64
	} else {
		text = text[129:len(text)] // 62+3+64
	}

	text = findWords(text)

	for i := 0; i < len(text); {
		var fcmpConditions map[string]map[string]string
		if strings.Contains(text[i].S, "FCMP") {
			fcmpConditions = dealWithFcmp(text[i].S)

			for fc, inst := range fcmpConditions {
				ops = append(ops, inst["op"])
				opstrs[fc] = inst["opstr"]
				instFormatComments[fc] = inst["instFormatComment"]
				instFormats[fc] = inst["instFormat"]
			}
			t := i + 1
			for ; text[t].Y == text[i].Y; t++ {
				continue
			}
			i = t
			continue
		}

		op := strings.Replace(text[i].S, ".", "_", -1)
		opstr := fmt.Sprintf("%s:\t\"%s\",", op, text[i].S)
		instFormatComment := ""

		if needSpecialHandle(op) {
			instFormatComment, instFormat, nextI := dealWithSpecialInst(op)
			ops = append(ops, op)
			opstrs[op] = opstr
			instFormatComments[op] = instFormatComment
			instFormats[op] = instFormat
			i += nextI
			continue
		}

		binValue := ""
		binMask := ""
		instArgs := ""
		j := i + 1
		for ; j < len(text) && text[j].Y == text[i].Y; j++ {

			// InstName Arg1, Arg2, Arg3...
			if res, _ := regexp.MatchString("^\\d+$", text[j].S); j == i+1 && res == false {
				instFormatComment = fmt.Sprintf("// %s %s", text[i].S, strings.Replace(text[j].S, ",", ", ", -1))
				continue
			}

			// binary code
			if res, _ := regexp.MatchString("^\\d+$", text[j].S); res {
				binValue += text[j].S
				binMask += strings.Repeat("1", len(text[j].S))
				// arg length
			} else {
				argLen, argName := getArg(op, text[j].S)
				addend := strings.Repeat("0", argLen)
				binValue += addend
				binMask += addend

				if instArgs != "" {
					instArgs = ", " + instArgs
				}
				instArgs = argName + instArgs
			}
		}

		ops = append(ops, op)
		opstrs[op] = opstr
		instFormatComments[op] = instFormatComment
		instFormat := fmt.Sprintf("{mask: %s, value: %s, op: %s, args: instArgs{%s}},", binstrToHex(binMask), binstrToHex(binValue), op, instArgs)
		instFormats[op] = instFormat

		i = j // next instruction
	}

	return
}
