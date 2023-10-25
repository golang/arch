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
package xmlspec

import (
	"encoding/xml"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
)

// warmUpCache initializes the XML decoding cache for the Instruction type.
// This is necessary because encoding/xml uses reflect to build a cache of
// struct fields, and this process is not thread-safe if multiple goroutines
// attempt to unmarshal into the same type for the first time concurrently.
func warmUpCache() {
	var inst Instruction
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

func ParseXMLFiles(dir string) []*Instruction {
	log.Println("Start parsing the xml files")
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	insts := make([]*Instruction, len(files))

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
func parse(f string) *Instruction {
	xmlFile, err := os.Open(f)
	if err != nil {
		log.Fatalf("Open file %s failed: %v\n", f, err)
	}
	defer xmlFile.Close()
	byteValue, err := io.ReadAll(xmlFile)
	if err != nil {
		log.Fatalf("io.ReadAll %s failed: %v\n", f, err)
	}

	var inst = new(Instruction)
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
