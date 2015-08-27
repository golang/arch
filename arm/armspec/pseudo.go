// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Lexer struct {
	input  string
	file   string
	lineno int
	sym    string
	prog   []*Stmt
}

type Line struct {
	File   string
	Lineno int
}

func (l Line) String() string {
	return fmt.Sprintf("%s:%d", l.File, l.Lineno)
}

var nerror int

func (l Line) Errorf(format string, args ...interface{}) {
	fmt.Printf("%s: %s\n", l, fmt.Sprintf(format, args...))
	if nerror++; nerror > 20 {
		fmt.Printf("too many errors\n")
		os.Exit(1)
	}
}

func re(s string) *regexp.Regexp {
	return regexp.MustCompile(`\A(?:` + s + `)`)
}

var tokens = []struct {
	re  *regexp.Regexp
	val int
	fn  func(*Lexer, string, *yySymType)
}{
	{re(`//[^\n]*`), -1, nil},
	{re(`/\*(.|\n)*?\*/`), -1, nil},
	{re(`[ \t\n]+`), -1, nil},
	{re(`»`), _INDENT, nil},
	{re(`«`), _UNINDENT, str},
	{re(`return`), _RETURN, str},
	{re(`UNDEFINED`), _UNDEFINED, str},
	{re(`UNPREDICTABLE`), _UNPREDICTABLE, str},
	{re(`SEE [^;]+`), _SEE, str},
	{re(`IMPLEMENTATION_DEFINED( [^;]+)?`), _IMPLEMENTATION_DEFINED, str},
	{re(`SUBARCHITECTURE_DEFINED( [^;]+)?`), _SUBARCHITECTURE_DEFINED, str},
	{re(`if`), _IF, nil},
	{re(`then`), _THEN, nil},
	{re(`repeat`), _REPEAT, nil},
	{re(`until`), _UNTIL, nil},
	{re(`while`), _WHILE, nil},
	{re(`case`), _CASE, nil},
	{re(`for`), _FOR, nil},
	{re(`to`), _TO, nil},
	{re(`do`), _DO, nil},
	{re(`of`), _OF, nil},
	{re(`elsif`), _ELSIF, nil},
	{re(`else`), _ELSE, nil},
	{re(`otherwise`), _OTHERWISE, nil},
	{re(`enumeration`), _ENUMERATION, nil},
	{re(`when`), _WHEN, nil},
	{re(`UNKNOWN`), _UNKNOWN, nil},
	{re(`DIV`), _DIV, nil},
	{re(`MOD`), _MOD, nil},
	{re(`AND`), _AND, nil},
	{re(`OR`), _OR, nil},
	{re(`EOR`), _EOR, nil},
	{re(`&&`), _ANDAND, nil},
	{re(`\|\|`), _OROR, nil},
	{re(`==`), _EQ, nil},
	{re(`!=`), _NE, nil},
	{re(` <`), _LT, nil},
	{re(` ?<=`), _LE, nil},
	{re(` ?>=`), _GE, nil},
	{re(` >`), _GT, nil},
	{re(`{`), '{', nil},
	{re(`}`), '}', nil},
	{re(`<`), '<', nil},
	{re(`>`), '>', nil},
	{re(`2^`), _TWOPOW, nil},
	{re(` ?<<`), _LSH, nil},
	{re(` ?>>`), _RSH, nil},
	{re(`,`), ',', nil},
	{re(`:`), ':', nil},
	{re(`\+`), '+', nil},
	{re(`\.`), '.', nil},
	{re(`-`), '-', nil},
	{re(`|`), '|', nil},
	{re(`\^`), '^', nil},
	{re(`\*`), '*', nil},
	{re(`/`), '/', nil},
	{re(`%`), '%', nil},
	{re(`&`), '&', nil},
	{re(`!`), '!', nil},
	{re(`;`), ';', nil},
	{re(`=`), '=', nil},
	{re(`\(`), '(', nil},
	{re(`\)`), ')', nil},
	{re(`\[`), '[', nil},
	{re(`\]`), ']', nil},
	{re(`!`), '!', nil},
	{re(`[0-9]+`), _CONST, str},
	{re(`[0-9]+\.[0-9]+`), _CONST, str},
	{re(`0x[0-9A-Fa-f]+`), _CONST, str},
	{re("[‘’][ 0-9x]+’"), _CONST, strNoSpaces},
	{re(`bit`), _BIT, str},
	{re(`bits\(`), _BITS, str1x},
	{re(`assert`), _ASSERT, str},
	{re(`integer`), _INTEGER, nil},
	{re(`boolean`), _BOOLEAN, nil},

	{re(`[A-Za-z_][A-Za-z0-9_]*`), _NAME, str},
	{re(`[A-Za-z_][A-Za-z0-9_]*\(`), _NAME_PAREN, str1x},
}

func (lx *Lexer) Lex(yy *yySymType) int {
	if len(lx.input) == 0 {
		return _EOF
	}
	var (
		longest    string
		longestVal int
		longestFn  func(*Lexer, string, *yySymType)
	)
	for _, tok := range tokens {
		s := tok.re.FindString(lx.input)
		if len(s) > len(longest) {
			longest = s
			longestVal = tok.val
			longestFn = tok.fn
		}
	}
	if longest == "" {
		lx.Error(fmt.Sprintf("lexer stuck at %.10q", lx.input))
		return -1
	}
	//println(longest)
	yy.line = lx.line()
	if longestFn != nil {
		lx.sym = longest
		longestFn(lx, longest, yy)
	}
	lx.input = lx.input[len(longest):]
	lx.lineno += strings.Count(longest, "\n")
	if longestVal < 0 {
		// skip
		return lx.Lex(yy)
	}
	return longestVal
}

func (lx *Lexer) Error(s string) {
	lx.line().Errorf("%s near %s", s, lx.sym)
}

func (lx *Lexer) line() Line {
	return Line{lx.file, lx.lineno}
}

func nop(*Lexer, string, *yySymType) {
	// having a function in the table
	// will make the lexer save the string
	// for use in error messages.
	// nothing more to do.
}

func str(lx *Lexer, s string, yy *yySymType) {
	yy.str = s
}

func str1(lx *Lexer, s string, yy *yySymType) {
	yy.str = s[1:]
}

func str1x(lx *Lexer, s string, yy *yySymType) {
	yy.str = s[:len(s)-1]
}

func strNoSpaces(lx *Lexer, s string, yy *yySymType) {
	yy.str = strings.Replace(s, " ", "", -1)
}

func parse(name, text string) []*Stmt {
	text = markup(text)
	lx := &Lexer{
		input:  text,
		file:   name,
		lineno: 1,
	}
	nerror = 0
	yyParse(lx)
	return lx.prog
}

func markup(text string) string {
	prefix := ""

	// Fix typos.
	text = strings.Replace(text, "R[i}", "R[i]", -1)
	text = strings.Replace(text, "R[n}", "R[n]", -1)
	text = strings.Replace(text, "(1 << (3-UInt(op)-UInt(size));", "(1 << (3-UInt(op)-UInt(size)));", -1)
	text = strings.Replace(text, "(D[n+r] AND NOT(D[m+r]);", "(D[n+r] AND NOT(D[m+r]));", -1)
	text = strings.Replace(text, "(D[d+r] AND NOT(D[m+r]);", "(D[d+r] AND NOT(D[m+r]));", -1)
	text = strings.Replace(text, "(D[n+r] AND D[d+r]) OR (D[m+r] AND NOT(D[d+r]);", "(D[n+r] AND D[d+r]) OR (D[m+r] AND NOT(D[d+r]));", -1)

	// Add indent, unindent tags.
	lines := strings.Split(text, "\n")
	var indent []int
	for j, line := range lines {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := 0
		for n < len(line) && line[n] == '\t' {
			n++
		}
		if len(indent) == 0 {
			indent = append(indent, n)
		}
		i := indent[len(indent)-1]
		if i > n {
			for i > n {
				line = "«" + line
				indent = indent[:len(indent)-1]
				if len(indent) == 0 {
					i = -1
				} else {
					i = indent[len(indent)-1]
				}
			}
			if i == -1 {
				indent = append(indent, n)
				i = n
			}
		}
		if i < n {
			line = "»" + line
			indent = append(indent, n)
		}
		lines[j] = line
	}
	n := len(indent) - 1
	if n < 0 {
		n = 0
	}
	return prefix + strings.Join(lines, "\n") + strings.Repeat("«", n)
}
