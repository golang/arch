// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// arm64gen parses Arm A-profile system register XML files and writes the
// system register encoding table used by the Go assembler.
//
// Download the system register XML from
// https://developer.arm.com/downloads/-/exploration-tools, extract it, and run:
//
//	arm64gen -i SysReg_xml_A_profile-2026-03 -o sysRegEnc.go
//
// The output belongs in $GOROOT/src/cmd/internal/obj/arm64.

package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Types corresponded to the data structures in the XML file.

type RegisterPage struct {
	XMLName   xml.Name  `xml:"register_page"`
	Registers Registers `xml:"registers"`
}

type Registers struct {
	XMLName  xml.Name `xml:"registers"`
	Register Register `xml:"register"`
}

type Register struct {
	XMLName          xml.Name         `xml:"register"`
	RegShortName     string           `xml:"reg_short_name"`
	RegVariables     RegVariables     `xml:"reg_variables"`
	AccessMechanisms AccessMechanisms `xml:"access_mechanisms"`
}

type RegVariables struct {
	XMLName     xml.Name    `xml:"reg_variables"`
	RegVariable RegVariable `xml:"reg_variable"`
}

type RegVariable struct {
	XMLName  xml.Name `xml:"reg_variable"`
	Variable string   `xml:"variable,attr"`
	Min      string   `xml:"min,attr"`
	Max      string   `xml:"max,attr"`
}

type AccessMechanisms struct {
	XMLName         xml.Name          `xml:"access_mechanisms"`
	AccessMechanism []AccessMechanism `xml:"access_mechanism"`
}

type AccessMechanism struct {
	XMLName  xml.Name `xml:"access_mechanism"`
	Accessor string   `xml:"accessor,attr"`
	Encoding Encoding `xml:"encoding"`
}

type Encoding struct {
	XMLName xml.Name `xml:"encoding"`
	Enc     []Enc    `xml:"enc"`
}

type Enc struct {
	XMLName xml.Name `xml:"enc"`
	V       string   `xml:"v,attr"`
}

type SystemReg struct {
	RegName        string
	EncBinary      uint32
	RegAccessFlags string
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

type accessFlag uint8

const (
	SR_READ accessFlag = 1 << iota
	SR_WRITE
)

func (a accessFlag) String() string {
	switch a {
	case SR_READ:
		return "SR_READ"
	case SR_WRITE:
		return "SR_WRITE"
	case SR_READ | SR_WRITE:
		return "SR_READ | SR_WRITE"
	default:
		return ""
	}
}

// encodingValue evaluates an XML encoding expression for register index n.
// Expressions are concatenations of binary constants and bit slices, for
// example "0b010:m[3]" and "m[2:0]".
func encodingValue(expr string, n int) (uint64, error) {
	var value uint64
	for len(expr) > 0 {
		if expr[0] == ':' {
			expr = expr[1:]
			continue
		}

		var part uint64
		var width int
		switch {
		case strings.HasPrefix(expr, "0b"):
			i := 2
			for i < len(expr) && expr[i] != ':' {
				if expr[i] != '0' && expr[i] != '1' {
					return 0, fmt.Errorf("unsupported encoding %q", expr)
				}
				i++
			}
			if i == 2 {
				return 0, fmt.Errorf("empty binary constant in %q", expr)
			}
			var err error
			part, err = strconv.ParseUint(expr[2:i], 2, 64)
			if err != nil {
				return 0, err
			}
			width = i - 2
			expr = expr[i:]

		case strings.HasPrefix(expr, "m[") || strings.HasPrefix(expr, "n["):
			end := strings.IndexByte(expr, ']')
			if end < 0 {
				return 0, fmt.Errorf("unterminated bit slice in %q", expr)
			}
			bits := strings.Split(expr[2:end], ":")
			hi, err := strconv.Atoi(bits[0])
			if err != nil {
				return 0, err
			}
			lo := hi
			if len(bits) == 2 {
				lo, err = strconv.Atoi(bits[1])
				if err != nil {
					return 0, err
				}
			} else if len(bits) != 1 {
				return 0, fmt.Errorf("invalid bit slice in %q", expr)
			}
			if hi < lo || lo < 0 {
				return 0, fmt.Errorf("invalid bit range %d:%d", hi, lo)
			}
			width = hi - lo + 1
			part = uint64(n>>lo) & (1<<width - 1)
			expr = expr[end+1:]

		default:
			return 0, fmt.Errorf("unsupported encoding %q", expr)
		}
		value = value<<width | part
	}
	return value, nil
}

func main() {
	// Write system register encoding to the sysRegEnc.go file.
	// This file should be put into $GOROOT/src/cmd/internal/obj/arm64/ directory.
	filename := flag.String("o", "sysRegEnc.go", "the name of the automatically generated file")
	xmlfolder := flag.String("i", "./files", "the folder where the data XML files are")
	flag.Parse()

	out, err := os.Create(*filename)
	check(err)
	defer out.Close()

	files, err := os.ReadDir(*xmlfolder)
	check(err)

	var systemregs []SystemReg
	regNum := 0

	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "AArch64-") || filepath.Ext(file.Name()) != ".xml" {
			continue
		}
		xmlFile, err := os.Open(filepath.Join(*xmlfolder, file.Name()))
		check(err)
		value, err := io.ReadAll(xmlFile)
		check(err)

		var regpage RegisterPage
		err = xml.Unmarshal(value, &regpage)
		if err != nil {
			log.Printf("%s: The data of this file does not fit into Register_page struct\n", file.Name())
			xmlFile.Close()
			continue
		}

		sysreg := regpage.Registers.Register
		sysregName := sysreg.RegShortName
		if strings.Contains(sysregName, "EL2") || strings.Contains(sysregName, "EL3") {
			log.Printf("%s: we do not support EL2 and EL3 system registers at the moment!\n", file.Name())
			xmlFile.Close()
			continue
		}
		if strings.Contains(sysregName, "<op1>_<Cn>_<Cm>_<op2>") {
			log.Printf("%s: The register %s is reserved\n", file.Name(), sysregName)
			xmlFile.Close()
			continue
		}
		if len(sysreg.AccessMechanisms.AccessMechanism) == 0 {
			log.Printf("%s: The data of this file does not fit into AccessMechanisms struct\n", file.Name())
			xmlFile.Close()
			continue
		}

		m := sysreg.AccessMechanisms.AccessMechanism
		var m0 *AccessMechanism
		for i := range m {
			if strings.Contains(m[i].Accessor, "MRS") || strings.Contains(m[i].Accessor, "MSR") {
				m0 = &m[i]
				break
			}
		}
		if m0 == nil {
			log.Printf("%s: \"%s\" is not a system register for MSR and MRS instructions.\n", file.Name(), sysregName)
			xmlFile.Close()
			continue
		}

		accessF := accessFlag(0)
		for j := range m {
			accessor := m[j].Accessor
			if strings.Contains(accessor, "MRS") {
				accessF |= SR_READ
			}
			if strings.Contains(accessor, "MSR") {
				accessF |= SR_WRITE
			}
		}
		aFlags := accessF.String()

		if len(m0.Encoding.Enc) != 5 {
			log.Printf("%s: The data of this file does not fit into S<op0>_<op1>_<Cn>_<Cm>_<op2> encoding\n", file.Name())
			xmlFile.Close()
			continue
		}

		min, max := 0, 0
		if strings.Contains(sysregName, "<n>") {
			if sysreg.RegVariables.RegVariable.Min != "" {
				min, err = strconv.Atoi(sysreg.RegVariables.RegVariable.Min)
				check(err)
			}
			max, err = strconv.Atoi(sysreg.RegVariables.RegVariable.Max)
			check(err)
		}
		var regs []SystemReg
		for n := min; n <= max; n++ {
			var enc [5]uint64
			valid := true
			for j := range m0.Encoding.Enc {
				enc[j], err = encodingValue(m0.Encoding.Enc[j].V, n)
				if err != nil {
					log.Printf("%s: %v", file.Name(), err)
					valid = false
					break
				}
			}
			if !valid {
				regs = nil
				break
			}
			name := strings.ReplaceAll(sysregName, "<n>", strconv.Itoa(n))
			binary := uint32(enc[0]<<19 | enc[1]<<16 | enc[2]<<12 | enc[3]<<8 | enc[4]<<5)
			regs = append(regs, SystemReg{name, binary, aFlags})
		}
		systemregs = append(systemregs, regs...)
		regNum += len(regs)
		// Close the xml file.
		xmlFile.Close()
	}
	log.Printf("The total number of parsing registers is %d\n", regNum)
	w := bufio.NewWriter(out)
	fmt.Fprintf(w, "// Code generated by arm64gen -i %s -o %s. DO NOT EDIT.\n", *xmlfolder, *filename)
	fmt.Fprintln(w, "\npackage arm64\n\nconst (\n\tSYSREG_BEGIN = REG_SPECIAL + iota")
	for i := 0; i < regNum; i++ {
		fmt.Fprintf(w, "\tREG_%s\n", systemregs[i].RegName)
	}
	fmt.Fprintln(w, "\tSYSREG_END\n)")
	fmt.Fprintln(w, `
const (
	SR_READ = 1 << iota
	SR_WRITE
)

var SystemReg = []struct {
	Name string
	Reg int16
	Enc uint32
	// AccessFlags is the readable and writeable property of system register.
	AccessFlags uint8
}{`)
	for i := 0; i < regNum; i++ {
		fmt.Fprintf(w, "\t{\"%s\", REG_%s, 0x%x, %s},\n", systemregs[i].RegName, systemregs[i].RegName, systemregs[i].EncBinary, systemregs[i].RegAccessFlags)
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w, `
func SysRegEnc(r int16) (string, uint32, uint8) {
	// The automatic generator guarantees that the order
	// of Reg in SystemReg struct is consistent with the
	// order of system register declarations
	if r <= SYSREG_BEGIN || r >= SYSREG_END {
		return "", 0, 0
	}
	v := SystemReg[r-SYSREG_BEGIN-1]
	return v.Name, v.Enc, v.AccessFlags
}`)
	w.Flush()
}
