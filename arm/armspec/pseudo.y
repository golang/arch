// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

%{
package main

import (
	"strconv"
)

%}

%union {
	str string
	line Line
	stmt *Stmt
	stmts []*Stmt
	expr *Expr
	exprs []*Expr
	elseifs []*ElseIf
	when *When
	whens []*When
	typ *Type
	typs []*Type
}

%token	_ASSERT
%token	_BITS
%token	_BIT
%token	_IF
%token	_EOF
%token	_NAME _NAME_PAREN
%token	_RETURN
%token	_UNDEFINED
%token	_UNPREDICTABLE
%token	_IMPLEMENTATION_DEFINED
%token	_SUBARCHITECTURE_DEFINED
%token	_ENUMERATION
%token	_DO
%token	_INDENT
%token	_UNINDENT
%token	_THEN
%token	_REPEAT
%token	_UNTIL
%token	_WHILE
%token	_CASE
%token	_FOR
%token	_TO
%token	_OF
%token	_ELSIF
%token	_ELSE
%token	_OTHERWISE
%token	_WHEN
%token	_CONST
%token	_UNKNOWN
%token	_EQ
%token	_NE
%token	_LE
%token	_GE
%token	_AND
%token	_OR
%token	_EOR
%token	_ANDAND
%token	_OROR
%token	_DIV
%token	_MOD
%token	_TWOPOW
%token	_LSH
%token	_RSH
%token	_INTEGER
%token	_BOOLEAN

%token	<str>	_NAME _NAME_PAREN _CONST
%token	<str>	_SEE

%left last_resort
%left '='
%left ','
%left _IF
%left _ANDAND _OROR
%left	_LT _LE '>' _GE _GT _EQ _NE
%left ':'
%left	'+' '-' '|' '^' _OR _EOR
%left	'*' '/' '%' '&' _LSH _RSH _DIV _MOD _AND
%left _TWOPOW
%left '.' '<' '['
%left	unary

%type	<stmts>	stmt_list simple_stmt_list stmt_list_opt
%type	<stmt>	stmt simple_stmt block simple_block otherwise enumeration
%type	<stmt>	else_opt simple_else_opt else_end simple_else_end
%type	<elseifs>	elsif_list simple_elsif_list
%type	<when>	when
%type	<whens>	when_list
%type	<exprs>	expr_list_opt expr_list expr_minus_list const_list
%type	<expr>	expr call_expr const
%type	<typ>	unnamed_type

%%

top:
	stmt_list_opt _EOF
	{
		yylex.(*Lexer).prog = $1
		return 0
	}

simple_stmt:
	expr '=' expr ';'
	{
		$$ = &Stmt{Op: Assign, X: $1, Y: $3}
	}
|	expr _EQ expr ';'
	{
		$$ = &Stmt{Op: Assign, X: $1, Y: $3}
	}
|	call_expr ';'
	{
		$$ = &Stmt{Op: StmtExpr, X: $1}
	}
|	_RETURN expr_list_opt ';'
	{
		$$ = &Stmt{Op: Return, List: $2}
	}
|	_UNDEFINED ';'
	{
		$$ = &Stmt{Op: Undefined}
	}
|	_UNPREDICTABLE ';'
	{
		$$ = &Stmt{Op: Unpredictable}
	}
|	_SEE ';'
	{
		$$ = &Stmt{Op: See, Text: $1}
	}
|	_IMPLEMENTATION_DEFINED ';'
	{
		$$ = &Stmt{Op: ImplDefined}
	}
|	_SUBARCHITECTURE_DEFINED ';'
	{
		$$ = &Stmt{Op: SubarchDefined}
	}
	
stmt:
	simple_stmt
|	unnamed_type _NAME ';'
	{
		$$ = &Stmt{Op: StmtExpr, X: &Expr{Op: Decl, Type: $1, Text: $2}}
	}
|	_IF expr _THEN block else_opt
	{
		$$ = &Stmt{Op: If, X: $2, Body: $4, ElseIf: $<elseifs>5, Else: $5}
	}
|	_IF expr _THEN simple_stmt simple_else_opt
	{
		$$ = &Stmt{Op: If, X: $2, Body: $4, ElseIf: $<elseifs>5, Else: $5}
	}
|	_REPEAT block _UNTIL expr ';'
	{
		$$ = &Stmt{Op: Repeat, Body: $2, X: $4}
	}
|	_WHILE expr do block
	{
		$$ = &Stmt{Op: While, X: $2, Body: $4}
	}
|	_FOR expr '=' expr _TO expr do block
	{
		$$ = &Stmt{Op: For, X: $2, Y: $4, Z: $6, Body: $8}
	}
|	_CASE expr _OF _INDENT when_list otherwise _UNINDENT
	{
		$$ = &Stmt{Op: Case, X: $2, When: $5, Else: $6}
	}
|	_ASSERT expr ';'
	{
		$$ = &Stmt{Op: Assert, X: $2}
	}
|	block
	{
		$$ = $1
	}
|	enumeration

enumeration:
	_ENUMERATION _NAME '{' expr_list '}' ';'
	{
		$$ = &Stmt{Op: Enum, Text: $2, List: $4}
	}

do:
|	_DO

block:
	_INDENT stmt_list _UNINDENT
	{
		$$ = &Stmt{Op: Block, Block: $2}
	}

simple_block:
	simple_stmt_list
	{
		$$ = &Stmt{Op: Block, Block: $1}
	}

simple_stmt_list:
	simple_stmt
	{
		$$ = []*Stmt{$1}
	}
|	simple_stmt_list simple_stmt
	{
		$$ = append($1, $2)
	}

stmt_list:
	stmt
	{
		$$ = []*Stmt{$1}
	}
|	stmt_list stmt
	{
		$$ = append($1, $2)
	}

stmt_list_opt:
	{
		$$ = nil
	}
|	stmt_list

else_opt:
	elsif_list else_end
	{
		$<elseifs>$ = $1
		$$ = $2
	}

simple_else_opt:
	simple_elsif_list simple_else_end
	{
		$<elseifs>$ = $1
		$$ = $2
	}

elsif_list:
	{
		$$ = nil
	}
|	elsif_list _ELSIF expr _THEN block
	{
		$$ = append($1, &ElseIf{Cond: $3, Body: $5})
	}

simple_elsif_list:
	{
		$$ = nil
	}
|	simple_elsif_list _ELSIF expr _THEN simple_stmt
	{
		$$ = append($1, &ElseIf{Cond: $3, Body: $5})
	}
	
else_end:
	{
		$$ = nil
	}
|	_ELSE block
	{
		$$ = $2
	}
|	_ELSE simple_stmt
	{
		$$ = $2
	}

simple_else_end:
	{
		$$ = nil
	}
|	_ELSE simple_stmt
	{
		$$ = $2
	}

when_list:
	{
		$$ = nil
	}
|	when_list when
	{
		$$ = append($1, $2)
	}

when:
	_WHEN const_list then block
	{
		$$ = &When{Cond: $2, Body: $4}
	}
|	_WHEN const_list then simple_block
	{
		$$ = &When{Cond: $2, Body: $4}
	}

then:
|	_THEN

otherwise:
	{
		$$ = nil
	}
|	_OTHERWISE block
	{
		$$ = $2
	}
|	_OTHERWISE simple_block
	{
		$$ = $2
	}

expr_list_opt:
	{
		$$ = nil
	}
|	expr_list

expr_list:
	expr
	{
		$$ = []*Expr{$1}
	}
|	expr_list ',' expr
	{
		$$ = append($1, $3)
	}

const_list:
	const
	{
		$$ = []*Expr{$1}
	}
|	const_list ',' const
	{
		$$ = append($1, $3)
	}

const:
	_CONST
	{
		$$ = &Expr{Op: Const, Text: $1}
	}
|	_NAME
	{
		$$ = &Expr{Op: Name, Text: $1}
	}

expr_minus_list:
	expr
	{
		$$ = []*Expr{$1}
	}
|	'-'
	{
		$$ = []*Expr{&Expr{Op: Blank}}
	}
|	expr_minus_list ',' expr
	{
		$$ = append($1, $3)
	}
|	expr_minus_list ',' '-'
	{
		$$ = append($1, &Expr{Op: Blank})
	}

expr:
	_CONST
	{
		$$ = &Expr{Op: Const, Text: $1}
	}
|	_NAME
	{
		$$ = &Expr{Op: Name, Text: $1}
	}
|	unnamed_type _NAME
	{
		$$ = &Expr{Op: Decl, Type: $1, Text: $2}
	}
|	_UNKNOWN
	{
		$$ = &Expr{Op: Unknown}
	}
|	unnamed_type _UNKNOWN
	{
		$$ = &Expr{Op: Unknown, Type: $1}
	}
|	call_expr
|	'(' expr_minus_list ')'
	{
		$$ = &Expr{Op: ExprTuple, List: $2}
	}
|	expr _EQ expr
	{
		$$ = &Expr{Op: Eq, X: $1, Y: $3}
	}
|	expr _NE expr
	{
		$$ = &Expr{Op: NotEq, X: $1, Y: $3}
	}
|	expr _LE expr
	{
		$$ = &Expr{Op: LtEq, X: $1, Y: $3}
	}
|	expr '<' expr_list '>'
	{
		$$ = &Expr{Op: BitIndex, X: $1, List: $3}
	}
|	expr _LT expr
	{
		$$ = &Expr{Op: Lt, X: $1, Y: $3}
	}
|	expr _GE expr
	{
		$$ = &Expr{Op: GtEq, X: $1, Y: $3}
	}
|	expr _GT expr
	{
		$$ = &Expr{Op: Gt, X: $1, Y: $3}
	}
|	_IF expr _THEN expr _ELSE expr %prec _IF
	{
		$$ = &Expr{Op: IfElse, X: $2, Y: $4, Z: $6}
	}
|	'!' expr %prec unary
	{
		$$ = &Expr{Op: Not, X: $2}
	}
|	expr _ANDAND expr
	{
		$$ = &Expr{Op: AndAnd, X: $1, Y: $3}
	}
|	expr _OROR expr
	{
		$$ = &Expr{Op: OrOr, X: $1, Y: $3}
	}
|	expr '^' expr
	{
		$$ = &Expr{Op: Eor, X: $1, Y: $3}
	}
|	expr ':' expr
	{
		$$ = &Expr{Op: Colon, X: $1, Y: $3}
	}
|	expr _AND expr
	{
		$$ = &Expr{Op: BigAND, X: $1, Y: $3}
	}
|	expr _OR expr
	{
		$$ = &Expr{Op: BigOR, X: $1, Y: $3}
	}
|	expr _EOR expr
	{
		$$ = &Expr{Op: BigEOR, X: $1, Y: $3}
	}
|	'+' expr %prec unary
	{
		$$ = &Expr{Op: Plus, X: $2}
	}
|	'-' expr %prec unary
	{
		$$ = &Expr{Op: Minus, X: $2}
	}
|	expr '+' expr
	{
		$$ = &Expr{Op: Add, X: $1, Y: $3}
	}
|	expr '-' expr
	{
		$$ = &Expr{Op: Sub, X: $1, Y: $3}
	}
|	expr '*' expr
	{
		$$ = &Expr{Op: Mul, X: $1, Y: $3}
	}
|	expr '/' expr
	{
		$$ = &Expr{Op: Div, X: $1, Y: $3}
	}
|	expr _DIV expr
	{
		$$ = &Expr{Op: BigDIV, X: $1, Y: $3}
	}
|	expr _MOD expr
	{
		$$ = &Expr{Op: BigMOD, X: $1, Y: $3}
	}
|	_TWOPOW expr
	{
		$$ = &Expr{Op: TwoPow, X: $2}
	}
|	expr _LSH expr
	{
		$$ = &Expr{Op: Lsh, X: $1, Y: $3}
	}
|	expr _RSH expr
	{
		$$ = &Expr{Op: Rsh, X: $1, Y: $3}
	}
|	expr '[' expr_list ']'
	{
		$$ = &Expr{Op: Index, X: $1, List: $3}
	}
|	expr '.' _NAME
	{
		$$ = &Expr{Op: Dot, X: $1, Text: $3}
	}
|	expr '=' expr %prec last_resort
	{
		$$ = &Expr{Op: Eq, X: $1, Y: $3}
	}

call_expr:
	_NAME_PAREN expr_list_opt ')'
	{
		$$ = &Expr{Op: Call, Text: $1, List: $2}
	}
	
unnamed_type:
	_BITS expr ')'
	{
		$$ = &Type{Op: BitType, NX: $2}
	}
|	_BIT
	{
		$$ = &Type{Op: BitType, N: 1}
	}
|	_INTEGER
	{
		$$ = &Type{Op: IntegerType}
	}
|	_BOOLEAN
	{
		$$ = &Type{Op: BoolType}
	}

%%

func parseIntConst(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
