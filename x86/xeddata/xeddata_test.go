// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"testing"
)

// Small database to generate state/xtype/width input files and validate parse results.
//
// Tests should use only those symbols that are defined inside test maps.
// For example, if {"foo"=>"bar"} element is not in statesMap, tests
// can't expect that "foo" get's replaced by "bar".
var (
	statesMap = map[string]string{
		"not64":         "MODE!=2",
		"mode64":        "MODE=2",
		"mode32":        "MODE=1",
		"mode16":        "MODE=0",
		"rexw_prefix":   "REXW=1 SKIP_OSZ=1",
		"norexw_prefix": "REXW=0 SKIP_OSZ=1",
		"W1":            "REXW=1 SKIP_OSZ=1",
		"W0":            "REXW=0 SKIP_OSZ=1",
		"VV1":           "VEXVALID=1",
		"V66":           "VEX_PREFIX=1",
		"VF2":           "VEX_PREFIX=2",
		"VF3":           "VEX_PREFIX=3",
		"V0F":           "MAP=1",
		"V0F38":         "MAP=2",
		"V0F3A":         "MAP=3",
		"VL128":         "VL=0",
		"VL256":         "VL=1",
	}

	xtypesMap = map[string]*xtype{
		"int": {name: "int", baseType: "INT", size: "0"},
		"i8":  {name: "i8", baseType: "INT", size: "8"},
		"i64": {name: "i64", baseType: "INT", size: "64"},
		"i32": {name: "i32", baseType: "INT", size: "32"},
		"u8":  {name: "u8", baseType: "UINT", size: "8"},
		"f32": {name: "f32", baseType: "SIGNLE", size: "32"},
		"f64": {name: "f64", baseType: "DOUBLE", size: "64"},
		"var": {name: "var", baseType: "VARIABLE", size: "0"},
	}

	widthsMap = map[string]*width{
		"q":         {xtype: "i64", sizes: [3]string{"8", "8", "8"}},
		"z":         {xtype: "int", sizes: [3]string{"2", "4", "4"}},
		"b":         {xtype: "u8", sizes: [3]string{"1", "1", "1"}},
		"d":         {xtype: "i32", sizes: [3]string{"4", "4", "4"}},
		"ps":        {xtype: "f32", sizes: [3]string{"16", "16", "16"}},
		"dq":        {xtype: "i32", sizes: [3]string{"16", "16", "16"}},
		"i32":       {xtype: "i32", sizes: [3]string{"4", "4", "4"}},
		"i64":       {xtype: "i64", sizes: [3]string{"8", "8", "8"}},
		"vv":        {xtype: "var", sizes: [3]string{"0", "0", "0"}},
		"mskw":      {xtype: "i1", sizes: [3]string{"64bits", "64bits", "64bits"}},
		"zf32":      {xtype: "f32", sizes: [3]string{"512bits", "512bits", "512bits"}},
		"zf64":      {xtype: "f64", sizes: [3]string{"512bits", "512bits", "512bits"}},
		"mem80real": {xtype: "f80", sizes: [3]string{"10", "10", "10"}},
		"mfpxenv":   {xtype: "struct", sizes: [3]string{"512", "512", "512"}},
	}
)

// newStatesSource returns a reader that mocks "all-state.txt" file.
// Input content is generated based on statesMap.
func newStatesSource() io.Reader {
	var buf bytes.Buffer
	i := 0
	for k, v := range statesMap {
		buf.WriteString("# Line comment\n")
		buf.WriteString("#\n\n\n")
		fmt.Fprintf(&buf, "\t%-20s%s", k, v)
		if i%3 == 0 {
			buf.WriteString("\t# Trailing comment")
		}
		buf.WriteByte('\n')
		i++
	}

	return &buf
}

// newWidthsSource returns a reader that mocks "all-widths.txt" file.
// Input content is generated based on widthsMap.
func newWidthsSource() io.Reader {
	var buf bytes.Buffer
	i := 0
	for name, width := range widthsMap {
		buf.WriteString("# Line comment\n")
		buf.WriteString("#\n\n\n")
		eqSizes := width.sizes[0] == width.sizes[1] &&
			width.sizes[0] == width.sizes[2]
		if i%2 == 0 && eqSizes {
			fmt.Fprintf(&buf, "\t%-16s%-12s%-8s",
				name, width.xtype, width.sizes[0])
		} else {
			fmt.Fprintf(&buf, "\t%-16s%-12s%-8s%-8s%-8s",
				name, width.xtype,
				width.sizes[0], width.sizes[1], width.sizes[2])
		}
		if i%3 == 0 {
			buf.WriteString("\t# Trailing comment")
		}
		buf.WriteByte('\n')
		i++
	}

	return &buf
}

// newXtypesSource returns a reader that mocks "all-element-types.txt" file.
// Input content is generated based on xtypesMap.
func newXtypesSource() io.Reader {
	var buf bytes.Buffer
	i := 0
	for _, v := range xtypesMap {
		buf.WriteString("# Line comment\n")
		buf.WriteString("#\n\n\n")

		fmt.Fprintf(&buf, "\t%s %s %s",
			v.name, v.baseType, v.size)

		if i%3 == 0 {
			buf.WriteString("\t# Trailing comment")
		}
		buf.WriteByte('\n')
		i++
	}

	return &buf
}

func newTestDatabase(t *testing.T) *Database {
	var db Database
	err := db.LoadStates(newStatesSource())
	if err != nil {
		t.Fatal(err)
	}
	err = db.LoadWidths(newWidthsSource())
	if err != nil {
		t.Fatal(err)
	}
	err = db.LoadXtypes(newXtypesSource())
	if err != nil {
		t.Fatal(err)
	}
	return &db
}

func TestContainsWord(t *testing.T) {
	tests := []struct {
		attrs    string
		attrName string
		output   bool
	}{
		{"ATT1", "ATT1", true},
		{" ATT1", "ATT1", true},
		{"ATT1 ", "ATT1", true},
		{" ATT1 ", "ATT1", true},
		{"ATT1 ATT2 ATT3", "ATT1", true},
		{"ATT1 ATT2 ATT3", "ATT2", true},
		{"ATT1 ATT2 ATT3", "ATT2", true},
		{"ATT1 ATT2 ATT3", "ATT4", false},
		{"ATT1ATT1", "ATT1", false},
		{".ATT1", "ATT1", false},
		{".ATT1.", "ATT1", false},
		{"ATT1.", "ATT1", false},
		{"", "ATT1", false},
		{"AT", "ATT1", false},
		{"ATT 1", "ATT1", false},
		{" ATT1 ", "TT", false},
		{" ATT1 ", "T1", false},
		{" ATT1 ", "AT", false},
	}

	for _, test := range tests {
		output := containsWord(test.attrs, test.attrName)
		if output != test.output {
			t.Errorf("containsWord(%q, %q)):\nhave: %v\nwant: %v",
				test.attrs, test.attrName, output, test.output)
		}
	}
}

func TestParseWidths(t *testing.T) {
	have, err := parseWidths(newWidthsSource())
	if err != nil {
		t.Fatal(err)
	}
	for k := range widthsMap {
		if have[k] == nil {
			t.Fatalf("missing key %s", k)
		}
		if *have[k] != *widthsMap[k] {
			t.Fatalf("key %s:\nhave: %#v\nwant: %#v",
				k, have[k], widthsMap[k])
		}
	}
	if !reflect.DeepEqual(have, widthsMap) {
		t.Errorf("widths output mismatch:\nhave: %#v\nwant: %#v",
			have, widthsMap)
	}
}

func TestParseStates(t *testing.T) {
	have, err := parseStates(newStatesSource())
	if err != nil {
		t.Fatal(err)
	}
	want := statesMap
	if !reflect.DeepEqual(have, want) {
		t.Errorf("states output mismatch:\nhave: %v\nwant: %v", have, want)
	}
}

func TestParseXtypes(t *testing.T) {
	have, err := parseXtypes(newXtypesSource())
	if err != nil {
		t.Fatal(err)
	}
	for k := range xtypesMap {
		if have[k] == nil {
			t.Fatalf("missing key %s", k)
		}
		if *have[k] != *xtypesMap[k] {
			t.Fatalf("key %s:\nhave: %#v\nwant: %#v",
				k, have[k], xtypesMap[k])
		}
	}
	if !reflect.DeepEqual(have, xtypesMap) {
		t.Fatalf("xtype maps are not equal")
	}
}

func TestNewOperand(t *testing.T) {
	tests := []struct {
		input string
		op    Operand
	}{
		// Simple cases.
		{
			"REG0=XMM_R():r",
			Operand{Name: "REG0=XMM_R()", Action: "r"},
		},
		{
			"REG0=XMM_R:w",
			Operand{Name: "REG0=XMM_R", Action: "w"},
		},
		{
			"MEM0:rw:q",
			Operand{Name: "MEM0", Action: "rw", Width: "q"},
		},
		{
			"REG0=XMM_R():rcw:ps:f32",
			Operand{Name: "REG0=XMM_R()", Action: "rcw", Width: "ps", Xtype: "f32"},
		},
		{
			"IMM0:r:z",
			Operand{Name: "IMM0", Action: "r", Width: "z"},
		},
		{
			"IMM1:cw:b:i8",
			Operand{Name: "IMM1", Action: "cw", Width: "b", Xtype: "i8"},
		},

		// Optional fields and visibility.
		{
			"REG2:r:EXPL",
			Operand{Name: "REG2", Action: "r", Visibility: VisExplicit},
		},
		{
			"MEM1:w:d:IMPL",
			Operand{Name: "MEM1", Action: "w", Width: "d", Visibility: VisImplicit},
		},
		{
			"MEM1:w:IMPL:d",
			Operand{Name: "MEM1", Action: "w", Width: "d", Visibility: VisImplicit},
		},
		{
			"MEM1:w:d:SUPP:i32",
			Operand{Name: "MEM1", Action: "w", Width: "d", Visibility: VisSuppressed, Xtype: "i32"},
		},
		{
			"MEM1:w:SUPP:d:i32",
			Operand{Name: "MEM1", Action: "w", Width: "d", Visibility: VisSuppressed, Xtype: "i32"},
		},

		// Ambiguity: xtypes that look like widths.
		{
			"REG0=XMM_R():w:dq:i64",
			Operand{Name: "REG0=XMM_R()", Action: "w", Width: "dq", Xtype: "i64"},
		},

		// TXT=X field.
		{
			"REG1=MASK1():r:mskw:TXT=ZEROSTR",
			Operand{Name: "REG1=MASK1()", Action: "r", Width: "mskw",
				Attributes: map[string]bool{"TXT=ZEROSTR": true}},
		},
		{
			"MEM0:r:vv:f64:TXT=BCASTSTR",
			Operand{Name: "MEM0", Action: "r", Width: "vv", Xtype: "f64",
				Attributes: map[string]bool{"TXT=BCASTSTR": true}},
		},
		{
			"REG0=ZMM_R3():w:zf32:TXT=SAESTR",
			Operand{Name: "REG0=ZMM_R3()", Action: "w", Width: "zf32",
				Attributes: map[string]bool{"TXT=SAESTR": true}},
		},
		{
			"REG0=ZMM_R3():w:zf64:TXT=ROUNDC",
			Operand{Name: "REG0=ZMM_R3()", Action: "w", Width: "zf64",
				Attributes: map[string]bool{"TXT=ROUNDC": true}},
		},

		// Multi-source.
		{
			"REG2=ZMM_N3():r:zf32:MULTISOURCE4",
			Operand{Name: "REG2=ZMM_N3()", Action: "r", Width: "zf32",
				Attributes: map[string]bool{"MULTISOURCE4": true}},
		},

		// Multi-source + EVEX.b context.
		{
			"REG2=ZMM_N3():r:zf32:MULTISOURCE4:TXT=SAESTR",
			Operand{Name: "REG2=ZMM_N3()", Action: "r", Width: "zf32",
				Attributes: map[string]bool{"MULTISOURCE4": true, "TXT=SAESTR": true}},
		},
	}

	db := newTestDatabase(t)
	for _, test := range tests {
		op, err := NewOperand(db, test.input)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*op, test.op) {
			t.Errorf("parse(`%s`): output mismatch\nhave: %#v\nwant: %#v",
				test.input, op, test.op,
			)
		}
	}
}

func TestReader(t *testing.T) {
	type test struct {
		name   string
		input  string
		output string
	}

	var tests []test
	{
		b, err := ioutil.ReadFile(path.Join("testdata", "xed_objects.txt"))
		if err != nil {
			t.Fatal(err)
		}
		cases := strings.Split(string(b), "------")[1:]
		for _, c := range cases {
			name := c[:strings.Index(c, "\n")]
			parts := strings.Split(c[len(name):], "====")

			tests = append(tests, test{
				name:   strings.TrimSpace(name),
				input:  strings.TrimSpace(parts[0]),
				output: strings.TrimSpace(parts[1]),
			})
		}
	}

	for _, test := range tests {
		r := NewReader(strings.NewReader(test.input))
		objects, err := r.ReadAll()
		if strings.Contains(test.name, "INVALID") {
			if err == nil {
				t.Errorf("%s: expected non-nil error", test.name)
				continue
			}
			if err.Error() != test.output {
				t.Errorf("%s: error mismatch\nhave: `%s`\nwant: `%s`\n",
					test.name, err.Error(), test.output)
			}
			t.Logf("PASS: %s", test.name)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}

		var have []map[string]string
		for _, o := range objects {
			for _, inst := range o.Insts {
				var result map[string]string
				err := json.Unmarshal([]byte(inst.String()), &result)
				if err != nil {
					t.Fatal(err)
				}
				have = append(have, result)
			}
		}
		var want []map[string]string
		err = json.Unmarshal([]byte(test.output), &want)
		if err != nil {
			t.Fatal(err)
		}
		for i := range want {
			for k := range want[i] {
				if want[i][k] == have[i][k] {
					continue
				}
				// i - index inside array of JSON objects.
				// k - i'th object key (example: "Iclass").
				t.Errorf("%s: insts[%d].%s mismatch\nhave: `%s`\nwant: `%s`",
					test.name, i, k, have[i][k], want[i][k])
			}
		}
		if !t.Failed() {
			t.Logf("PASS: %s", test.name)
		}
	}
}

func TestMacroExpand(t *testing.T) {
	tests := [...]struct {
		input  string
		output string
	}{
		0: {
			"a not64 b c",
			"a MODE!=2 b c",
		},
		1: {
			"mode16 W0",
			"MODE=0 REXW=0 SKIP_OSZ=1",
		},
		2: {
			"W1 mode32",
			"REXW=1 SKIP_OSZ=1 MODE=1",
		},
		3: {
			"W1 W1",
			"REXW=1 SKIP_OSZ=1 REXW=1 SKIP_OSZ=1",
		},
		4: {
			"W1W1",
			"W1W1",
		},
		5: {
			"mode64 1 2 3 rexw_prefix",
			"MODE=2 1 2 3 REXW=1 SKIP_OSZ=1",
		},
		6: {
			"a  b  c",
			"a b c",
		},
		7: {
			"mode16 mode32 mode16 mode16",
			"MODE=0 MODE=1 MODE=0 MODE=0",
		},
		8: {
			"V0F38 V0FV0F V0FV0F38",
			"MAP=2 V0FV0F V0FV0F38",
		},
		9: {
			"VV1 0x2E V66 V0F38 VL128  norexw_prefix MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()",
			"VEXVALID=1 0x2E VEX_PREFIX=1 MAP=2 VL=0 REXW=0 SKIP_OSZ=1 MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()",
		},
	}

	db := newTestDatabase(t)
	for id, test := range tests {
		have := ExpandStates(db, test.input)
		if test.output != have {
			t.Errorf("test %d: output mismatch:\nhave: `%s`\nwant: `%s`",
				id, have, test.output)
		}
	}
}
