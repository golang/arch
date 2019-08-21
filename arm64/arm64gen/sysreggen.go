// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is an automatic parser program that parses arm64
// system register XML files to get the encoding information
// and writes them to the sysRegEnc.go file. The sysRegEnc.go
// file is used for the system register encoding.
// Follow the following steps to run the automatic parser program:
// 1. The system register XML files are from
// https://developer.arm.com/-/media/Files/ATG/Beta10/SysReg_xml_v85A-2019-06.tar.gz
// 2. Extract SysReg_xml_v85A-2019-06.tar/SysReg_xml_v85A-2019-06/SysReg_xml_v85A-2019-06/AArch64-*.xml
// to a "xmlfolder" folder.
// 3. Run the command: ./sysrengen -i "xmlfolder" -o "filename"
// By default, the xmlfolder is "./files" and the filename is "sysRegEnc.go".
// 4. Put the automaically generated file into $GOROOT/src/cmd/internal/obj/arm64 directory.

package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
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

func main() {
	// Write system register encoding to the sysRegEnc.go file.
	// This file should be put into $GOROOT/src/cmd/internal/obj/arm64/ directory.
	filename := flag.String("o", "sysRegEnc.go", "the name of the automatically generated file")
	xmlfolder := flag.String("i", "./files", "the folder where the data XML files are")
	flag.Parse()

	out, err := os.Create(*filename)
	check(err)
	defer out.Close()

	files, err := ioutil.ReadDir(*xmlfolder)
	check(err)

	var systemregs []SystemReg
	regNum := 0

	for _, file := range files {
		xmlFile, err := os.Open(filepath.Join(*xmlfolder, file.Name()))
		check(err)
		value, err := ioutil.ReadAll(xmlFile)
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

		m0 := sysreg.AccessMechanisms.AccessMechanism[0]
		ins := m0.Accessor
		if !(strings.Contains(ins, "MRS") || strings.Contains(ins, "MSR")) {
			log.Printf("%s: \"%s\" is not a system register for MSR and MRS instructions.\n", file.Name(), sysregName)
			xmlFile.Close()
			continue
		}

		m := sysreg.AccessMechanisms.AccessMechanism
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

		max := 0
		var enc [5]uint64
		if len(m0.Encoding.Enc) != 5 {
			log.Printf("%s: The data of this file does not fit into S<op0>_<op1>_<Cn>_<Cm>_<op2> encoding\n", file.Name())
			xmlFile.Close()
			continue
		}
		// Special handling for system register name containing <n>.
		if strings.Contains(sysregName, "<n>") {
			max, err = strconv.Atoi(sysreg.RegVariables.RegVariable.Max)
			check(err)
			for n := 0; n <= max; n++ {
				name := strings.Replace(sysregName, "<n>", strconv.Itoa(n), -1)
				systemregs = append(systemregs, SystemReg{name, 0, aFlags})
				regNum++
			}
		} else {
			systemregs = append(systemregs, SystemReg{sysregName, 0, aFlags})
			regNum++
		}
		for i := 0; i <= max; i++ {
			index := regNum - 1 - max + i
			for j := 0; j < len(m0.Encoding.Enc); j++ {
				value := m0.Encoding.Enc[j].V
				// value="0b010:n[3]"
				// value="0b1:n[1:0]"
				// value="ob10:n[4:3]"
				if strings.Contains(value, "n") && strings.Contains(value, "b") {
					v0 := strings.Split(value, "b")
					v1 := strings.Split(v0[1], "n")
					v2 := strings.Trim(v1[1], "[]")
					bits, err := strconv.ParseUint(strings.Trim(v1[0], ":"), 2, 32)
					check(err)
					if strings.Contains(v1[1], ":") {
						// v1[1]="[1:0]", v2="1:0"
						// Get the index.
						first, err := strconv.Atoi(strings.Split(v2, ":")[0])
						check(err)
						last, err := strconv.Atoi(strings.Split(v2, ":")[1])
						check(err)
						// Get the corresponding appended bits.
						bitsAppend := (i >> uint(last) & (1<<uint(first-last+1) - 1))
						// Join the bits to get the final bits.
						finalBits := int(bits)<<uint(first-last+1) | bitsAppend
						enc[j] = uint64(finalBits)
					} else {
						// v1[1]="[3]", v2="3"
						// Get the corresponding appended bits.
						first, err := strconv.Atoi(v2)
						check(err)
						bitsAppend := (i >> uint(first)) & 1
						// Join the bits to get the final bits.
						finalBits := int(bits)<<1 | bitsAppend
						enc[j] = uint64(finalBits)
					}
				} else if strings.Contains(value, "n") && !strings.Contains(value, "b") {
					// value="n[3:0]" | value="n[2:0]"
					v0 := strings.Split(value, "n")
					v1 := strings.Trim(v0[1], "[]")
					v2 := strings.Split(v1, ":")
					// Convert string format to integer.
					first, err := strconv.Atoi(v2[0])
					check(err)
					last, err := strconv.Atoi(v2[1])
					check(err)
					finalBits := (i >> uint(last) & (1<<uint(first-last+1) - 1))
					enc[j] = uint64(finalBits)
				} else {
					// value="0b110"
					v := strings.Split(value, "b")
					var err error = nil
					enc[j], err = strconv.ParseUint(v[1], 2, 64)
					check(err)
				}
			}
			systemregs[index].EncBinary = uint32(enc[0]<<19 | enc[1]<<16 | enc[2]<<12 | enc[3]<<8 | enc[4]<<5)
		}
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
