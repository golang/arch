// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unify

import (
	"fmt"
	"iter"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A Domain is a non-empty set of values, all of the same kind.
//
// Domain may be a scalar:
//
//   - [String] - Represents string-typed values.
//
// Or a composite:
//
//   - [Def] - A mapping from fixed keys to [Domain]s.
//
//   - [Tuple] - A fixed-length sequence of [Domain]s or
//     all possible lengths repeating a [Domain].
//
// Or top or bottom:
//
//   - [Top] - Represents all possible values of all kinds.
//
//   - nil - Represents no values.
//
// Or a variable:
//
//   - [Var] - A value captured in the environment.
type Domain interface {
	Exact() bool

	// decode stores this value in a Go value. If this value is not exact, this
	// returns a potentially wrapped *inexactError.
	decode(reflect.Value) error
}

type inexactError struct {
	valueType string
	goType    string
}

func (e *inexactError) Error() string {
	return fmt.Sprintf("cannot store inexact %s value in %s", e.valueType, e.goType)
}

type decodeError struct {
	path string
	err  error
}

func newDecodeError(path string, err error) *decodeError {
	if err, ok := err.(*decodeError); ok {
		return &decodeError{path: path + "." + err.path, err: err.err}
	}
	return &decodeError{path: path, err: err}
}

func (e *decodeError) Unwrap() error {
	return e.err
}

func (e *decodeError) Error() string {
	return fmt.Sprintf("%s: %s", e.path, e.err)
}

// Top represents all possible values of all possible types.
type Top struct{}

func (t Top) Exact() bool { return false }

func (t Top) decode(rv reflect.Value) error {
	// We can decode Top into a pointer-typed value as nil.
	if rv.Kind() != reflect.Pointer {
		return &inexactError{"top", rv.Type().String()}
	}
	rv.SetZero()
	return nil
}

// A Def is a mapping from field names to [Value]s. Any fields not explicitly
// listed have [Value] [Top].
type Def struct {
	fields map[string]*Value
}

// NewDef creates a new [Def].
//
// The fields and values slices must have the same length.
func NewDef(fields []string, values []*Value) Def {
	if len(fields) != len(values) {
		panic("fields and values must have the same length")
	}
	m := make(map[string]*Value, len(fields))
	for i := range fields {
		if _, ok := m[fields[i]]; ok {
			panic(fmt.Sprintf("duplicate field %q", fields[i]))
		}
		m[fields[i]] = values[i]
	}
	return Def{m}
}

// Exact returns true if all field Values are exact.
func (d Def) Exact() bool {
	for _, v := range d.fields {
		if !v.Exact() {
			return false
		}
	}
	return true
}

func (d Def) decode(rv reflect.Value) error {
	rv, err := preDecode(rv, reflect.Struct, "Def")
	if err != nil {
		return err
	}
	var lowered map[string]string // Lower case -> canonical for d.fields.
	rt := rv.Type()
	for fi := range rv.NumField() {
		fType := rt.Field(fi)
		if fType.PkgPath != "" {
			continue
		}
		v := d.fields[fType.Name]
		if v == nil {
			v = topValue

			// Try a case-insensitive match
			canon, ok := d.fields[strings.ToLower(fType.Name)]
			if ok {
				v = canon
			} else {
				if lowered == nil {
					lowered = make(map[string]string, len(d.fields))
					for k := range d.fields {
						l := strings.ToLower(k)
						if k != l {
							lowered[l] = k
						}
					}
				}
				canon, ok := lowered[strings.ToLower(fType.Name)]
				if ok {
					v = d.fields[canon]
				}
			}
		}
		if err := v.Domain.decode(rv.Field(fi)); err != nil {
			return newDecodeError(fType.Name, err)
		}
	}
	return nil
}

func (d Def) keys() []string {
	return slices.Sorted(maps.Keys(d.fields))
}

func (d Def) All() iter.Seq2[string, *Value] {
	// TODO: We call All fairly often. It's probably bad to sort this every
	// time.
	keys := slices.Sorted(maps.Keys(d.fields))
	return func(yield func(string, *Value) bool) {
		for _, k := range keys {
			if !yield(k, d.fields[k]) {
				return
			}
		}
	}
}

// A Tuple is a sequence of Values in one of two forms: 1. a fixed-length tuple,
// where each Value can be different or 2. a "repeated tuple", which is a Value
// repeated 0 or more times.
type Tuple struct {
	vs []*Value

	// repeat, if non-nil, means this Tuple consists of an element repeated 0 or
	// more times. If repeat is non-nil, vs must be nil. This is a generator
	// function because we don't necessarily want *exactly* the same Value
	// repeated. For example, in YAML encoding, a !sum in a repeated tuple needs
	// a fresh variable in each instance.
	repeat []func(nonDetEnv) (*Value, nonDetEnv)
}

func NewTuple(vs ...*Value) Tuple {
	return Tuple{vs: vs}
}

func NewRepeat(gens ...func(nonDetEnv) (*Value, nonDetEnv)) Tuple {
	return Tuple{repeat: gens}
}

func (d Tuple) Exact() bool {
	if d.repeat != nil {
		return false
	}
	for _, v := range d.vs {
		if !v.Exact() {
			return false
		}
	}
	return true
}

func (d Tuple) decode(rv reflect.Value) error {
	if d.repeat != nil {
		return &inexactError{"repeated tuple", rv.Type().String()}
	}
	// TODO: We could also do arrays.
	rv, err := preDecode(rv, reflect.Slice, "Tuple")
	if err != nil {
		return err
	}
	if rv.IsNil() || rv.Cap() < len(d.vs) {
		rv.Set(reflect.MakeSlice(rv.Type(), len(d.vs), len(d.vs)))
	} else {
		rv.SetLen(len(d.vs))
	}
	for i, v := range d.vs {
		if err := v.Domain.decode(rv.Index(i)); err != nil {
			return newDecodeError(fmt.Sprintf("%d", i), err)
		}
	}
	return nil
}

// A String represents a set of strings. It can represent the intersection of a
// set of regexps, or a single exact string. In general, the domain of a String
// is non-empty, but we do not attempt to prove emptiness of a regexp value.
type String struct {
	kind  stringKind
	re    []*regexp.Regexp // Intersection of regexps
	exact string
}

type stringKind int

const (
	stringRegex stringKind = iota
	stringExact
)

func NewStringRegex(exprs ...string) (String, error) {
	if len(exprs) == 0 {
		exprs = []string{""}
	}
	v := String{kind: -1}
	for _, expr := range exprs {
		re, err := regexp.Compile(`\A(?:` + expr + `)\z`)
		if err != nil {
			return String{}, fmt.Errorf("parsing value: %s", err)
		}

		// An exact value narrows the whole domain to exact, so we're done, but
		// should keep parsing.
		if v.kind == stringExact {
			continue
		}

		if _, complete := re.LiteralPrefix(); complete {
			v = String{kind: stringExact, exact: expr}
		} else {
			v.kind = stringRegex
			v.re = append(v.re, re)
		}
	}
	return v, nil
}

func NewStringExact(s string) String {
	return String{kind: stringExact, exact: s}
}

// Exact returns whether this Value is known to consist of a single string.
func (d String) Exact() bool {
	return d.kind == stringExact
}

func (d String) decode(rv reflect.Value) error {
	if d.kind != stringExact {
		return &inexactError{"regex", rv.Type().String()}
	}
	rv2, err := preDecode(rv, reflect.String, "String")
	if err == nil {
		rv2.SetString(d.exact)
		return nil
	}
	rv2, err = preDecode(rv, reflect.Int, "String")
	if err == nil {
		i, err := strconv.Atoi(d.exact)
		if err != nil {
			return fmt.Errorf("cannot decode String into %s: %s", rv.Type(), err)
		}
		rv2.SetInt(int64(i))
		return nil
	}
	return err
}
