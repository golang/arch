// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Types for XED enum-like constants.
type (
	// OperandSizeMode describes operand size mode (66H prefix).
	OperandSizeMode int

	// AddressSizeMode describes address size mode (67H prefix).
	AddressSizeMode int

	// CPUMode describes availability in certain CPU mode.
	CPUMode int
)

// Possible operand size modes. XED calls it OSZ.
const (
	OpSize16 OperandSizeMode = iota
	OpSize32
	OpSize64
)

// Possible address size modes. XED calls it ASZ.
const (
	AddrSize16 AddressSizeMode = iota
	AddrSize32
	AddrSize64
)

// Possible CPU modes. XED calls it MODE.
const (
	Mode16 CPUMode = iota
	Mode32
	Mode64
)

var sizeStrings = [...]string{"16", "32", "64"}

// sizeString maps size enumeration value to it's string representation.
func sizeString(size int) string {
	// Panic more gracefully than with "index out of range".
	// If client code specified invalid size enumeration,
	// this is programming error that should be fixed, not "handled".
	if size >= len(sizeStrings) {
		panic(fmt.Sprintf("illegal size value: %d", size))
	}
	return sizeStrings[size]
}

// String returns osz bit size string. Panics on illegal enumerations.
func (osz OperandSizeMode) String() string { return sizeString(int(osz)) }

// String returns asz bit size string. Panics on illegal enumerations.
func (asz AddressSizeMode) String() string { return sizeString(int(asz)) }

// Database holds information that is required to
// properly handle XED datafiles.
type Database struct {
	widths map[string]*width // all-widths.txt
	states map[string]string // all-state.txt
	xtypes map[string]*xtype // all-element-types.txt
}

// width is a "all-width.txt" record.
type width struct {
	// Default xtype name (examples: int, i8, f32).
	xtype string

	// 16, 32 and 64 bit sizes (all may have same value).
	sizes [3]string
}

// xtype is a "all-element-type.txt" record.
type xtype struct {
	// Name is xtype identifier.
	name string

	// baseType specifies xtype base type.
	// See "all-element-type-base.txt".
	baseType string

	// Size is an operand data size in bits.
	size string
}

// NewDatabase returns Database that loads everything
// it can find in xedPath.
// Missing lookup file is not an error, but error during
// parsing of found file is.
//
// Lookup:
//	"$xedPath/all-state.txt" => db.LoadStates()
//	"$xedPath/all-widths.txt" => db.LoadWidths()
//	"$xedPath/all-element-types.txt" => db.LoadXtypes()
// $xedPath is the interpolated value of function argument.
//
// The call NewDatabase("") is valid and returns empty database.
// Load methods can be used to read lookup files one-by-one.
func NewDatabase(xedPath string) (*Database, error) {
	var db Database

	stat, err := os.Stat(xedPath)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("xedPath is not directory")
	}

	states, err := os.Open(filepath.Join(xedPath, "all-state.txt"))
	if err == nil {
		err = db.LoadStates(states)
		if err != nil {
			return &db, err
		}
	}

	widths, err := os.Open(filepath.Join(xedPath, "all-widths.txt"))
	if err == nil {
		err = db.LoadWidths(widths)
		if err != nil {
			return &db, err
		}
	}

	xtypes, err := os.Open(filepath.Join(xedPath, "all-element-types.txt"))
	if err == nil {
		err = db.LoadXtypes(xtypes)
		if err != nil {
			return &db, err
		}
	}

	return &db, nil
}

// LoadWidths reads XED widths definitions from r and updates db.
// "widths" are 16/32/64 bit mode type sizes.
// See "$XED/obj/dgen/all-widths.txt".
func (db *Database) LoadWidths(r io.Reader) error {
	var err error
	db.widths, err = parseWidths(r)
	return err
}

// LoadStates reads XED states definitions from r and updates db.
// "states" are simple macro substitutions without parameters.
// See "$XED/obj/dgen/all-state.txt".
func (db *Database) LoadStates(r io.Reader) error {
	var err error
	db.states, err = parseStates(r)
	return err
}

// LoadXtypes reads XED xtypes definitions from r and updates db.
// "xtypes" are low-level XED type names.
// See "$XED/obj/dgen/all-element-types.txt".
// See "$XED/obj/dgen/all-element-type-base.txt".
func (db *Database) LoadXtypes(r io.Reader) error {
	var err error
	db.xtypes, err = parseXtypes(r)
	return err
}

// WidthSize translates width string to size string using desired
// SizeMode m. For some widths output is the same for any valid value of m.
func (db *Database) WidthSize(width string, m OperandSizeMode) string {
	info := db.widths[width]
	if info == nil {
		return ""
	}
	return info.sizes[m]
}

func parseWidths(r io.Reader) (map[string]*width, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse widths: %v", err)
	}

	// Lines have two forms:
	// 1. name xtype size [# comment]
	// 2. name xtype size16, size32, size64 [# comment]
	reLine := regexp.MustCompile(`(^\s*\w+\s+\w+\s+\w+\s+\w+\s+\w+)|(^\s*\w+\s+\w+\s+\w+)`)

	widths := make(map[string]*width, 128)
	for _, l := range bytes.Split(data, []byte("\n")) {
		var name, xtype, size16, size32, size64 string

		if m := reLine.FindSubmatch(l); m != nil {
			var f [][]byte
			if m[1] != nil {
				f = bytes.Fields(m[1])
			} else {
				f = bytes.Fields(m[2])
			}

			name = string(f[0])
			xtype = string(f[1])
			if len(f) > 3 {
				size16 = string(f[2])
				size32 = string(f[3])
				size64 = string(f[4])
			} else {
				size16 = string(f[2])
				size32 = size16
				size64 = size16
			}
		}
		if name != "" {
			widths[name] = &width{
				xtype: xtype,
				sizes: [3]string{size16, size32, size64},
			}
		}
	}

	return widths, nil
}

func parseStates(r io.Reader) (map[string]string, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse states: %v", err)
	}

	// Lines have form of "name ...replacements [# comment]".
	// This regexp captures the name and everything until line end or comment.
	lineRE := regexp.MustCompile(`^\s*(\w+)\s+([^#]+)`)

	states := make(map[string]string, 128)
	for _, l := range strings.Split(string(data), "\n") {
		if m := lineRE.FindStringSubmatch(l); m != nil {
			name, replacements := m[1], m[2]
			states[name] = strings.TrimSpace(replacements)
		}
	}

	return states, nil
}

func parseXtypes(r io.Reader) (map[string]*xtype, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse xtypes: %v", err)
	}

	// Lines have form of "name baseType size [# comment]".
	lineRE := regexp.MustCompile(`^\s*(\w+)\s+(\w+)\s*(\d+)`)

	xtypes := make(map[string]*xtype)
	for _, l := range strings.Split(string(data), "\n") {
		if m := lineRE.FindStringSubmatch(l); m != nil {
			name, baseType, size := m[1], m[2], m[3]
			xtypes[name] = &xtype{
				name:     name,
				baseType: baseType,
				size:     size,
			}
		}
	}

	return xtypes, nil
}
