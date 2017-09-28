// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"sort"
)

// generateAenum generates instruction ID enumeration.
// Adds elements from newNames if they are not already there.
// Output enum entries are sorted by their name (except ALAST
// which is always the last element).
//
// Reader - old/current "aenum.go" contents provider.
// Writer - new "aenum.go" contents consumer.
//
// Reads r to examine current A-enum (instruction IDs prefixed with "A")
// file contents. Updated contents are written to w.
func generateAenum(r io.Reader, w io.Writer, newNames []string) error {
	f, fset, err := parseFile(r)
	if err != nil {
		return err
	}

	decl := removeAenumDecl(f)
	if decl == nil {
		return errors.New(filenameAenum + " missing AXXX const decl clause")
	}
	last := decl.Specs[len(decl.Specs)-1]
	decl.Specs = decl.Specs[:len(decl.Specs)-1] // Drop "ALAST".
	for _, name := range newNames {
		decl.Specs = append(decl.Specs, &ast.ValueSpec{
			Names: []*ast.Ident{{Name: "A" + name}},
		})
	}
	sort.Slice(decl.Specs, func(i, j int) bool {
		x, y := decl.Specs[i].(*ast.ValueSpec), decl.Specs[j].(*ast.ValueSpec)
		return x.Names[0].Name < y.Names[0].Name
	})
	decl.Specs = append(decl.Specs, last)

	// Reset nodes positions.
	for _, spec := range decl.Specs {
		spec := spec.(*ast.ValueSpec)
		resetPos(spec)
		if spec.Doc != nil {
			return fmt.Errorf("%s: doc comments are not supported", spec.Names[0].Name)
		}
		if spec.Comment != nil {
			resetPos(spec.Comment)
		}
	}

	var buf bytes.Buffer
	format.Node(&buf, fset, f)
	buf.WriteByte('\n')
	format.Node(&buf, fset, decl)

	// Additional formatting call is needed to make
	// whitespace gofmt-compliant.
	prettyCode, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	w.Write(prettyCode)

	return nil
}

// removeAenumDecl searches AXXX constand decl and removes it from f.
// Associated comments are also removed.
// Returns AXXX declaration or nil, if it was not found.
func removeAenumDecl(f *ast.File) *ast.GenDecl {
	for i, decl := range f.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if decl.Tok != token.CONST {
			continue
		}
		// AXXX enum is distinguished by trailing ALAST.
		last := decl.Specs[len(decl.Specs)-1].(*ast.ValueSpec)
		if len(last.Names) == 1 && last.Names[0].Name == "ALAST" {
			// Remove comments.
			blacklist := make(map[*ast.CommentGroup]bool)
			if decl.Doc != nil {
				blacklist[decl.Doc] = true
			}
			for _, spec := range decl.Specs {
				spec := spec.(*ast.ValueSpec)
				if spec.Doc != nil {
					blacklist[spec.Doc] = true
				}
				if spec.Comment != nil {
					blacklist[spec.Comment] = true
				}
			}
			comments := f.Comments[:0]
			for _, c := range f.Comments {
				if !blacklist[c] {
					comments = append(comments, c)
				}
			}
			f.Comments = comments
			// Remove decl itself.
			f.Decls = append(f.Decls[:i], f.Decls[i+1:]...)

			return decl
		}
	}

	return nil
}

// reset node position info.
func resetPos(node ast.Node) {
	switch node := node.(type) {
	case *ast.CommentGroup:
		node.List[0].Slash = 0
	case *ast.ValueSpec:
		node.Names[0].NamePos = 0
	default:
		panic(fmt.Sprintf("can't reset pos for %T", node))
	}
}

// parseFile parses file that is identified by specified path.
func parseFile(r io.Reader) (*ast.File, *token.FileSet, error) {
	src, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	mode := parser.ParseComments
	f, err := parser.ParseFile(fset, filenameAenum, src, mode)
	if err != nil {
		return nil, nil, err
	}
	return f, fset, nil
}
