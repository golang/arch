// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unify

import (
	"fmt"
	"iter"
	"reflect"
)

// A Value represents a structured, non-deterministic value consisting of
// strings, tuples of Values, and string-keyed maps of Values. A
// non-deterministic Value will also contain variables, which are resolved via
// an environment as part of a [Closure].
//
// For debugging, a Value can also track the source position it was read from in
// an input file, and its provenance from other Values.
type Value struct {
	Domain Domain

	// A Value has either a pos or parents (or neither).
	pos     *Pos
	parents *[2]*Value
}

var (
	topValue    = &Value{Domain: Top{}}
	bottomValue = &Value{Domain: nil}
)

// NewValue returns a new [Value] with the given domain and no position
// information.
func NewValue(d Domain) *Value {
	return &Value{Domain: d}
}

// NewValuePos returns a new [Value] with the given domain at position p.
func NewValuePos(d Domain, p Pos) *Value {
	return &Value{Domain: d, pos: &p}
}

// newValueFrom returns a new [Value] with the given domain that copies the
// position information of p.
func newValueFrom(d Domain, p *Value) *Value {
	return &Value{Domain: d, pos: p.pos, parents: p.parents}
}

func unified(d Domain, p1, p2 *Value) *Value {
	return &Value{Domain: d, parents: &[2]*Value{p1, p2}}
}

func (v *Value) Pos() Pos {
	if v.pos == nil {
		return Pos{}
	}
	return *v.pos
}

func (v *Value) PosString() string {
	var b []byte
	for root := range v.Provenance() {
		if len(b) > 0 {
			b = append(b, ' ')
		}
		b, _ = root.pos.AppendText(b)
	}
	return string(b)
}

func (v *Value) Exact() bool {
	if v.Domain == nil {
		return false
	}
	return v.Domain.Exact()
}

// Decode decodes v into a Go value.
//
// v must be exact, except that it can include Top. into must be a pointer.
// [Def]s are decoded into structs. [Tuple]s are decoded into slices. [String]s
// are decoded into strings or ints. Any field can itself be a pointer to one of
// these types. Top can be decoded into a pointer-typed field and will set the
// field to nil. Anything else will allocate a value if necessary.
func (v *Value) Decode(into any) error {
	rv := reflect.ValueOf(into)
	if rv.Kind() != reflect.Pointer {
		return fmt.Errorf("cannot decode into non-pointer %T", into)
	}
	return v.Domain.decode(rv)
}

func preDecode(rv reflect.Value, kind reflect.Kind, name string) (reflect.Value, error) {
	if rv.Kind() == kind {
		return rv, nil
	}
	if rv.Kind() == reflect.Pointer && rv.Type().Elem().Kind() == kind {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return rv.Elem(), nil
	}
	return reflect.Value{}, fmt.Errorf("cannot decode %s into %s", name, rv.Type())
}

// Provenance iterates over all of the source Values that have contributed to
// this Value.
func (v *Value) Provenance() iter.Seq[*Value] {
	return func(yield func(*Value) bool) {
		var rec func(d *Value) bool
		rec = func(d *Value) bool {
			if d.pos != nil {
				if !yield(d) {
					return false
				}
			}
			if d.parents != nil {
				for _, p := range d.parents {
					if !rec(p) {
						return false
					}
				}
			}
			return true
		}
		rec(v)
	}
}
