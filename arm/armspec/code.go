// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var errSeeOther = fmt.Errorf("see other")
var errStop = fmt.Errorf("stop")
var errUndefined = fmt.Errorf("undefined")

type StmtOp int

const (
	_      StmtOp = iota
	Assign        // 1
	Return
	Undefined
	Unpredictable
	See
	ImplDefined
	SubarchDefined
	If
	Repeat
	While // 10
	For
	Case
	Enum
	Block
	StmtExpr
	Fndef
	Assert
)

type Stmt struct {
	Op      StmtOp
	X, Y, Z *Expr
	List    []*Expr
	Text    string
	Body    *Stmt
	Else    *Stmt
	When    []*When
	ElseIf  []*ElseIf
	Block   []*Stmt
	Type    *Type
}

type When struct {
	Cond []*Expr
	Body *Stmt
}

type ElseIf struct {
	Cond *Expr
	Body *Stmt
}

type ExprOp int

const (
	_     ExprOp = iota
	Blank        // 1
	Const
	Name
	Decl
	Unknown
	Call
	ExprTuple
	Eq
	NotEq
	LtEq // 10
	Lt
	GtEq
	Gt
	BitIndex
	IfElse
	Not
	AndAnd
	OrOr
	Eor
	Colon // 20
	And
	Or
	Plus
	Minus
	Add
	Sub
	Mul
	Div
	BigDIV
	BigMOD // 30
	BigAND
	BigOR
	BigEOR
	TwoPow
	Lsh
	Rsh
	Index
	Dot
)

type Expr struct {
	Op      ExprOp
	Text    string
	X, Y, Z *Expr
	List    []*Expr
	Type    *Type
}

type TypeOp int

const (
	_ TypeOp = iota
	BoolType
	BitType
	IntegerType
	NamedType
	TupleType
)

type Type struct {
	Op   TypeOp
	List []*Type
	N    int
	NX   *Expr
	Text string
}

type Exec struct {
	Vars map[string]Value
}

type Inconsistent struct {
}

func (Inconsistent) String() string {
	return "INCONSISTENT"
}

type Bits struct {
	N        int
	Val      uint32
	DontCare uint32
}

func (b Bits) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "`")
	for i := b.N - 1; i >= 0; i-- {
		if (b.DontCare>>uint(i))&1 != 0 {
			fmt.Fprintf(&buf, "x")
		} else if (b.Val>>uint(i))&1 != 0 {
			fmt.Fprintf(&buf, "1")
		} else {
			fmt.Fprintf(&buf, "0")
		}
	}
	fmt.Fprintf(&buf, "'")
	return buf.String()
}

func (b Bits) Eq(v Value) bool {
	c := v.(Bits)
	if b.N != c.N {
		panic(fmt.Errorf("compare of mismatched bit lengths %v and %v", b, c))
	}
	return b.Val&^c.DontCare == c.Val
}

func (b Bits) Lt(v Value) bool {
	panic("less than on bits")
}

func (b Bits) Gt(v Value) bool {
	panic("greater than on bits")
}

func isValid(inst *Inst, w uint32) bool {
	var ctxt Exec
	ctxt.Vars = make(map[string]Value)
	off := 0
	for _, b := range strings.Split(inst.Bits, "|") {
		wid := 1
		if i := strings.Index(b, ":"); i >= 0 {
			wid, _ = strconv.Atoi(b[i+1:])
			b = b[:i]
		}
		switch b {
		case "1", "(1)":
			if (w>>uint(31-off))&1 != 1 {
				return false
			}
		case "0", "(0)":
			if (w>>uint(31-off))&1 != 0 {
				return false
			}
		default:
			bits := Bits{N: wid, Val: (w >> uint(32-off-wid)) & (1<<uint(wid) - 1)}
			//	fmt.Print(b, " ", bits)
			if old, ok := ctxt.Vars[b]; ok && old != bits {
				ctxt.Define(b, Inconsistent{})
			}
			ctxt.Define(b, bits)
		}
		off += wid
	}
	//	fmt.Println()

	for _, stmt := range inst.Prog {
		err := ctxt.stmt(stmt)
		if err != nil {
			if err == errSeeOther || err == errUndefined {
				return false
			}
			if err == errStop {
				break
			}
			panic(err)
		}
	}
	return true
}

func (ctxt *Exec) Define(name string, value Value) {
	ctxt.Vars[name] = value
}

func (ctxt *Exec) Assign(name string, value Value) error {
	ctxt.Vars[name] = value
	return nil
}

/*

var global = map[string]Value{
	"ConditionPassed": _ConditionPassed,
	"UInt": _UInt,
	"SInt": _SInt,
}

type Defn struct {
	Name string
	Value Value
	Next *Defn
}

func (ctxt *Exec) Assign(name string, value Value) error {
	for d := ctxt.Scope; d != nil; d = d.Next {
		if d.Name == name {
			d.Value = value
			return nil
		}
	}
	ctxt.Define(name, value)
	return nil
}

func _ConditionPassed(ctxt *Exec, args []Value) (Value, error) {
	return Bool(true), nil
}
*/

type Int int64

func (x Int) String() string {
	return fmt.Sprint(int64(x))
}

func (x Int) Eq(y Value) bool { return x == y }
func (x Int) Lt(y Value) bool { return x < y.(Int) }
func (x Int) Gt(y Value) bool { return x > y.(Int) }

func (x Int) Add(y Value) Value { return x + y.(Int) }
func (x Int) Sub(y Value) Value { return x - y.(Int) }
func (x Int) Mul(y Value) Value { return x * y.(Int) }

func (x Int) Lsh(y Value) Value { return x << uint(y.(Int)) }
func (x Int) Rsh(y Value) Value { return x >> uint(y.(Int)) }
func (x Int) DIV(y Value) Value { return x / y.(Int) }

func _UInt(_ *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("UInt takes a single argument")
	}
	b, ok := args[0].(Bits)
	if !ok {
		return nil, fmt.Errorf("UInt takes a Bits, not %T", args[0])
	}
	if b.N > 63 {
		return nil, fmt.Errorf("UInt cannot handle %d-bit Bits", b.N)
	}
	return Int(b.Val), nil
}

func _SInt(_ *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("SInt takes a single argument")
	}
	b, ok := args[0].(Bits)
	if !ok {
		return nil, fmt.Errorf("SInt takes a Bits, not %T", args[0])
	}
	if b.N > 64 {
		return nil, fmt.Errorf("SInt cannot handle %d-bit Bits", b.N)
	}
	return Int(int64(b.Val) << uint(64-b.N) >> uint(64-b.N)), nil
}

/*

func (ctxt *Exec) Run(prog []*Stmt) error {
	for _, stmt := range prog {
		if err := ctxt.stmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

*/

func (ctxt *Exec) stmt(stmt *Stmt) error {
	switch stmt.Op {
	case If:
		v, err := toBool(ctxt.expr(stmt.X))
		if err != nil {
			return err
		}
		if v {
			return ctxt.stmt(stmt.Body)
		}
		for _, elseif := range stmt.ElseIf {
			v, err := toBool(ctxt.expr(elseif.Cond))
			if err != nil {
				return err
			}
			if v {
				return ctxt.stmt(elseif.Body)
			}
		}
		if stmt.Else == nil {
			return nil
		}
		return ctxt.stmt(stmt.Else)

	case Case:
		v, err := ctxt.expr(stmt.X)
		if err != nil {
			return err
		}
		vv, ok := v.(interface {
			Eq(Value) bool
		})
		if !ok {
			return fmt.Errorf("use of uncomparable value %T(%v) in case statement", v, v)
		}
		for _, when := range stmt.When {
			for _, cond := range when.Cond {
				w, err := ctxt.expr(cond)
				if err != nil {
					return err
				}
				if reflect.TypeOf(v) != reflect.TypeOf(w) {
					return fmt.Errorf("mistyped comparison of %T(%v) and %T(%v) in case statement", v, v, w, w)
				}
				if vv.Eq(w) {
					return ctxt.stmt(when.Body)
				}
			}
		}
		if stmt.Else == nil {
			return nil
		}
		return ctxt.stmt(stmt.Else)

	case Block:
		for _, x := range stmt.Block {
			if err := ctxt.stmt(x); err != nil {
				return err
			}
		}
		return nil

	case See:
		return errSeeOther

	case Undefined:
		return errUndefined

	case Unpredictable, ImplDefined, SubarchDefined:
		return errStop

	case StmtExpr:
		_, err := ctxt.expr(stmt.X)
		return err

	case Assign:
		v, err := ctxt.expr(stmt.Y)
		if err != nil {
			return err
		}
		if stmt.X.Op == ExprTuple {
			vv, ok := v.(Tuple)
			if !ok {
				return fmt.Errorf("assignment of non-tuple %T to tuple", v)
			}
			if len(stmt.X.List) != len(vv) {
				return fmt.Errorf("%d = %d in tuple assignment", len(stmt.X.List), len(vv))
			}
			for i, x := range stmt.X.List {
				if x.Op == Blank {
					continue
				}
				if x.Op != Name {
					return fmt.Errorf("cannot assign to expr op %d", x.Op)
				}
				if err := ctxt.Assign(x.Text, vv[i]); err != nil {
					return err
				}
			}
			return nil
		}
		x := stmt.X
		if x.Op != Name {
			return fmt.Errorf("cannot assign to expr op %d", x.Op)
		}
		return ctxt.Assign(x.Text, v)
	}
	return fmt.Errorf("unknown stmt op %d", stmt.Op)
}

func toBool(v Value, err error) (b Bool, xerr error) {
	if err != nil {
		return false, err
	}
	switch v := v.(type) {
	case Bool:
		return v, nil
	default:
		return false, fmt.Errorf("value of type %T used as bool", v)
	}
}

type Value interface {
	String() string
}

type Bool bool

func (b Bool) Eq(v Value) bool { return b == v }

func (b Bool) Not() Value { return !b }

func (b Bool) String() string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

type Tuple []Value

func (t Tuple) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "(")
	for i, v := range t {
		if i > 0 {
			fmt.Fprintf(&buf, ", ")
		}
		fmt.Fprintf(&buf, v.String())
	}
	fmt.Fprintf(&buf, ")")
	return buf.String()
}

func (ctxt *Exec) expr(x *Expr) (v Value, err error) {
	switch x.Op {
	case Call:
		fn, err := ctxt.name(x.Text)
		if err != nil {
			return nil, err
		}
		var list []Value
		for _, y := range x.List {
			v, err := ctxt.expr(y)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return ctxt.call(x.Text, fn, list)

	case ExprTuple:
		var list []Value
		for _, y := range x.List {
			v, err := ctxt.expr(y)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		if len(list) == 1 {
			return list[0], nil
		}
		return Tuple(list), nil

	case AndAnd:
		v, err := toBool(ctxt.expr(x.X))
		if err != nil {
			return nil, err
		}
		if !v {
			return v, nil
		}
		return ctxt.expr(x.Y)

	case OrOr:
		v, err := toBool(ctxt.expr(x.X))
		if err != nil {
			return nil, err
		}
		if v {
			return v, nil
		}
		return ctxt.expr(x.Y)

	case Colon:
		v, err := ctxt.expr(x.X)
		if err != nil {
			return nil, err
		}
		y, err := ctxt.expr(x.Y)
		if err != nil {
			return nil, err
		}
		xb, ok := v.(Bits)
		yb, ok2 := y.(Bits)
		if !ok || !ok2 {
			return nil, fmt.Errorf("colon operator requires bit strings")
		}
		b := xb
		b.N += yb.N
		b.Val <<= uint(yb.N)
		b.DontCare <<= yb.DontCare
		return b, nil

	case Name:
		return ctxt.name(x.Text)

	case Const:
		if (strings.HasPrefix(x.Text, "‘") || strings.HasPrefix(x.Text, "’")) && strings.HasSuffix(x.Text, "’") {
			text := x.Text[len("‘") : len(x.Text)-len("’")]
			var b Bits
			b.N = len(text)
			for _, c := range text {
				b.Val <<= 1
				b.DontCare <<= 1
				if c == '1' {
					b.Val |= 1
				}
				if c == 'x' {
					b.DontCare |= 1
				}
			}
			return b, nil
		}
		n, err := strconv.Atoi(x.Text)
		if err == nil {
			return Int(n), nil
		}
		println("const", x.Text)

	case Not:
		l, err := ctxt.expr(x.X)
		if err != nil {
			return nil, err
		}
		switch x.Op {
		case Not:
			ll, ok := l.(interface {
				Not() Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support !", l)
			}
			return ll.Not(), nil
		}

	case Eq, NotEq, Lt, LtEq, Gt, GtEq, Add, Sub, Mul, Lsh, Rsh, BigDIV:
		l, err := ctxt.expr(x.X)
		if err != nil {
			return nil, err
		}
		r, err := ctxt.expr(x.Y)
		if err != nil {
			return nil, err
		}
		tl := reflect.TypeOf(l)
		tr := reflect.TypeOf(r)
		if tl != tr {
			return nil, fmt.Errorf("arithmetic (expr op %d) of %T(%v) with %T(%v)", x.Op, l, l, r, r)
		}
		switch x.Op {
		case Eq:
			ll, ok := l.(interface {
				Eq(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support ==", l)
			}
			return Bool(ll.Eq(r)), nil
		case NotEq:
			ll, ok := l.(interface {
				Eq(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support !=", l)
			}
			return Bool(!ll.Eq(r)), nil
		case Lt:
			ll, ok := l.(interface {
				Lt(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support <", l)
			}
			return Bool(ll.Lt(r)), nil
		case GtEq:
			ll, ok := l.(interface {
				Lt(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support >=", l)
			}
			return Bool(!ll.Lt(r)), nil
		case Gt:
			ll, ok := l.(interface {
				Gt(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support >", l)
			}
			return Bool(ll.Gt(r)), nil
		case LtEq:
			ll, ok := l.(interface {
				Gt(Value) bool
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support <=", l)
			}
			return Bool(!ll.Gt(r)), nil
		case Add:
			ll, ok := l.(interface {
				Add(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support +", l)
			}
			return ll.Add(r), nil
		case Sub:
			ll, ok := l.(interface {
				Sub(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support -", l)
			}
			return ll.Sub(r), nil
		case Mul:
			ll, ok := l.(interface {
				Mul(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support *", l)
			}
			return ll.Mul(r), nil
		case Lsh:
			ll, ok := l.(interface {
				Lsh(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support <<", l)
			}
			return ll.Lsh(r), nil
		case Rsh:
			ll, ok := l.(interface {
				Rsh(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support >>", l)
			}
			return ll.Rsh(r), nil
		case BigDIV:
			ll, ok := l.(interface {
				DIV(Value) Value
			})
			if !ok {
				return nil, fmt.Errorf("type %T does not support DIV", l)
			}
			return ll.DIV(r), nil
		}

	case BitIndex:
		l, err := ctxt.expr(x.X)
		if err != nil {
			return nil, err
		}
		b, ok := l.(Bits)
		if !ok {
			return nil, fmt.Errorf("bit index operator requires bitstring, not %T(%v)", l, l)
		}
		out := Bits{}
		for _, ix := range x.List {
			if ix.Op == Colon {
				r1, err := ctxt.expr(ix.X)
				if err != nil {
					return nil, err
				}
				r2, err := ctxt.expr(ix.Y)
				if err != nil {
					return nil, err
				}
				i1, ok := r1.(Int)
				i2, ok2 := r2.(Int)
				if !ok || !ok2 {
					return nil, fmt.Errorf("bit indexes must be int")
				}
				if i1 <= i2 {
					return nil, fmt.Errorf("inverted bit indexes %d:%d", i1, i2)
				}
				w := int(i1 + 1 - i2)
				out.N += w
				out.Val <<= uint(w)
				out.DontCare <<= uint(w)
				out.Val |= (b.Val >> uint(i2)) & (1<<uint(w) - 1)
				out.DontCare |= (b.DontCare >> uint(i2)) & (1<<uint(w) - 1)
			} else {
				r, err := ctxt.expr(ix)
				if err != nil {
					return nil, err
				}
				i, ok := r.(Int)
				if !ok {
					return nil, fmt.Errorf("bit index operator index must be int")
				}
				out.N++
				out.Val <<= 1
				out.DontCare <<= 1
				out.Val |= (b.Val >> uint(i)) & 1
			}
		}
		return out, nil

	case IfElse:
		v, err := toBool(ctxt.expr(x.X))
		if err != nil {
			return nil, err
		}
		if v {
			return ctxt.expr(x.Y)
		}
		return ctxt.expr(x.Z)
	}
	return nil, fmt.Errorf("unknown expr op %d", x.Op)
}

type Func struct {
	Name string
	F    func(*Exec, []Value) (Value, error)
}

func (f Func) String() string {
	return f.Name
}

func (ctxt *Exec) call(name string, fn Value, args []Value) (Value, error) {
	switch fn := fn.(type) {
	case Func:
		return fn.F(ctxt, args)
	}
	return nil, fmt.Errorf("cannot call %s of type %T", name, fn)
}

var global = map[string]Value{
	"UInt":           Func{"UInt", _UInt},
	"DecodeImmShift": Func{"DecodeImmShift", _DecodeImmShift},
	"ArchVersion":    Func{"ArchVersion", _ArchVersion},
	"ZeroExtend":     Func{"ZeroExtend", _ZeroExtend},
	"ARMExpandImm":   Func{"ARMExpandImm", _ARMExpandImm},
	"Zeros":          Func{"Zeros", _Zeros},
	"TRUE":           Bool(true),
	"FALSE":          Bool(false),
	"BitCount":       Func{"BitCount", _BitCount},
	"Consistent":     Func{"Consistent", _Consistent},
}

func _Consistent(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("BitCount requires one argument")
	}
	_, inconsistent := args[0].(Inconsistent)
	return Bool(!inconsistent), nil
}

func _BitCount(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("BitCount requires one argument")
	}
	b, ok1 := args[0].(Bits)
	if !ok1 {
		return nil, fmt.Errorf("BitCount requires bitstring argument")
	}

	n := 0
	for i := 0; i < b.N; i++ {
		if b.Val&(1<<uint(i)) != 0 {
			n++
		}
	}
	return Int(n), nil
}

func _ArchVersion(ctxt *Exec, args []Value) (Value, error) {
	return Int(7), nil
}

func _ZeroExtend(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("ZeroExtend requires two arguments")
	}
	b, ok := args[0].(Bits)
	n, ok2 := args[1].(Int)
	if !ok || !ok2 {
		return nil, fmt.Errorf("DecodeImmShift requires bitstring, int arguments")
	}
	b.N = int(n)
	return b, nil
}

func _DecodeImmShift(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("DecodeImmShift requires two arguments")
	}
	b1, ok1 := args[0].(Bits)
	b2, ok2 := args[1].(Bits)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("DecodeImmShift requires bitstring arguments")
	}
	_ = b1
	_ = b2
	// TODO
	return Tuple{Int(0), Int(0)}, nil
}

func _ARMExpandImm(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ARMExpandImm requires one argument")
	}
	b, ok1 := args[0].(Bits)
	if !ok1 || b.N != 12 {
		return nil, fmt.Errorf("ARMExpandImm requires 12-bit bitstring argument")
	}
	v := uint32(b.Val & 0xFF)
	rot := uint(2 * ((b.Val >> 8) & 0xF))
	v = v>>rot | v<<(32-rot)
	return Bits{N: 32, Val: v}, nil
}

func _Zeros(ctxt *Exec, args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Zeros requires one argument")
	}
	n, ok := args[0].(Int)
	if !ok {
		return nil, fmt.Errorf("Zeros requires int argument")
	}
	return Bits{N: int(n)}, nil
}

type Symbol string

func (s Symbol) String() string { return string(s) }

func (ctxt *Exec) name(name string) (v Value, err error) {
	v, ok := ctxt.Vars[name]
	if ok {
		return v, nil
	}
	v, ok = global[name]
	if ok {
		return v, nil
	}
	return Symbol(name), nil
	return nil, fmt.Errorf("unknown name %s", name)
}

/*
func pseudoExec(base uint32, enc *Enc) {
	var ctxt Exec
	ctxt.Define("EncodingSpecificOperations", func(ctxt *Exec, args []Value) (Value, error) {
		return nil, ctxt.Run(enc.Prog)
	})

	var n uint
	for _, f := range enc.Fields {
		switch f {
		case "0", "1", "(0)", "(1)":
			n++
		default:
			wid := size[f]
			if wid == 0 {
				panic("missing width for " + f)
			}
			ctxt.Define(f, Bits{N: wid, Val: (base>>(31-n))&(1<<uint(wid)-1)})
			n += uint(wid)
		}
	}

	if err := ctxt.Run(enc.Inst.Prog); err != nil {
		log.Printf("%#x: %v", base, err)
	}
}

func loadLibrary(data []byte) {
	prog := parse("speclib.txt", string(data))
	for _, stmt := range prog {
		switch stmt.Op {
		default:
			log.Fatalf("unexpected statement in speclib.txt: %d", stmt.Op)
		case Fndef:
			global[stmt.Text] = funcImpl(stmt)
		case Enum:
			// TODO
		}
	}
}

func funcImpl(stmt *Stmt) func(*Exec, []Value) (Value, error) {
	return func(ctxt *Exec, args []Value) (Value, error) {
		ctxt1 := *ctxt
		if len(args) != len(stmt.List) {
			return nil, fmt.Errorf("calling %s: have %d arguments, want %d", stmt.Text, len(args), len(stmt.List))
		}
		for i, decl := range stmt.List {
			v, err := convert(args[i], decl.Type)
			if err != nil {
				return nil, fmt.Errorf("calling %s: %v", stmt.Text, err)
			}
			ctxt1.Define(decl.Text, v)
		}
		err := ctxt1.stmt(stmt.Body)
		if err != nil {
			return nil, err
		}
		if ctxt1.ret == nil && stmt.Type != nil {
			return nil, fmt.Errorf("calling %s: function body missing return", stmt.Text)
		}
		if ctxt1.ret != nil && stmt.Type == nil {
			return nil, fmt.Errorf("calling %s: unexpected return value from function with no result", stmt.Text)
		}
		return ctxt1.ret, nil
	}
}

func convert(v Value, typ *Type) (Value, error) {
	switch typ.Op {
	case BoolType:
		if v, ok := v.(Bool); ok {
			return v, nil
		}
	case BitType:
		if v, ok := v.(Bits); ok && v.N == typ.N {
			return v, nil
		}
	}
	return nil, fmt.Errorf("cannot convert %s to type %v", v, typ)
}

*/
