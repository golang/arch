//line pseudo.y:2
package main

import __yyfmt__ "fmt"

//line pseudo.y:2
import (
	"strconv"
)

//line pseudo.y:10
type yySymType struct {
	yys     int
	str     string
	line    Line
	stmt    *Stmt
	stmts   []*Stmt
	expr    *Expr
	exprs   []*Expr
	elseifs []*ElseIf
	when    *When
	whens   []*When
	typ     *Type
	typs    []*Type
}

const _ASSERT = 57346
const _BITS = 57347
const _BIT = 57348
const _IF = 57349
const _EOF = 57350
const _NAME = 57351
const _NAME_PAREN = 57352
const _RETURN = 57353
const _UNDEFINED = 57354
const _UNPREDICTABLE = 57355
const _IMPLEMENTATION_DEFINED = 57356
const _SUBARCHITECTURE_DEFINED = 57357
const _ENUMERATION = 57358
const _DO = 57359
const _INDENT = 57360
const _UNINDENT = 57361
const _THEN = 57362
const _REPEAT = 57363
const _UNTIL = 57364
const _WHILE = 57365
const _CASE = 57366
const _FOR = 57367
const _TO = 57368
const _OF = 57369
const _ELSIF = 57370
const _ELSE = 57371
const _OTHERWISE = 57372
const _WHEN = 57373
const _CONST = 57374
const _UNKNOWN = 57375
const _EQ = 57376
const _NE = 57377
const _LE = 57378
const _GE = 57379
const _AND = 57380
const _OR = 57381
const _EOR = 57382
const _ANDAND = 57383
const _OROR = 57384
const _DIV = 57385
const _MOD = 57386
const _TWOPOW = 57387
const _LSH = 57388
const _RSH = 57389
const _INTEGER = 57390
const _BOOLEAN = 57391
const _SEE = 57392
const last_resort = 57393
const _LT = 57394
const _GT = 57395
const unary = 57396

var yyToknames = []string{
	"_ASSERT",
	"_BITS",
	"_BIT",
	"_IF",
	"_EOF",
	"_NAME",
	"_NAME_PAREN",
	"_RETURN",
	"_UNDEFINED",
	"_UNPREDICTABLE",
	"_IMPLEMENTATION_DEFINED",
	"_SUBARCHITECTURE_DEFINED",
	"_ENUMERATION",
	"_DO",
	"_INDENT",
	"_UNINDENT",
	"_THEN",
	"_REPEAT",
	"_UNTIL",
	"_WHILE",
	"_CASE",
	"_FOR",
	"_TO",
	"_OF",
	"_ELSIF",
	"_ELSE",
	"_OTHERWISE",
	"_WHEN",
	"_CONST",
	"_UNKNOWN",
	"_EQ",
	"_NE",
	"_LE",
	"_GE",
	"_AND",
	"_OR",
	"_EOR",
	"_ANDAND",
	"_OROR",
	"_DIV",
	"_MOD",
	"_TWOPOW",
	"_LSH",
	"_RSH",
	"_INTEGER",
	"_BOOLEAN",
	"_SEE",
	"last_resort",
	" =",
	" ,",
	"_LT",
	" >",
	"_GT",
	" :",
	" +",
	" -",
	" |",
	" ^",
	" *",
	" /",
	" %",
	" &",
	" .",
	" <",
	" [",
	"unary",
}
var yyStatenames = []string{}

const yyEofCode = 1
const yyErrCode = 2
const yyMaxDepth = 200

//line pseudo.y:544
func parseIntConst(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

//line yacctab:1
var yyExca = []int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyNprod = 107
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 1272

var yyAct = []int{

	15, 190, 103, 44, 16, 134, 194, 16, 42, 5,
	47, 48, 49, 50, 140, 4, 13, 137, 79, 39,
	78, 138, 43, 6, 85, 46, 6, 181, 154, 134,
	133, 16, 96, 89, 91, 92, 93, 94, 79, 75,
	55, 74, 139, 134, 84, 83, 101, 82, 168, 81,
	6, 80, 108, 109, 110, 111, 79, 113, 114, 115,
	116, 117, 118, 119, 120, 121, 122, 123, 124, 125,
	126, 127, 128, 129, 130, 79, 112, 76, 134, 102,
	153, 77, 203, 3, 100, 188, 40, 27, 63, 150,
	16, 93, 132, 70, 71, 131, 72, 73, 143, 144,
	145, 16, 39, 147, 179, 180, 149, 142, 41, 6,
	41, 86, 68, 69, 141, 202, 75, 55, 74, 95,
	148, 98, 53, 54, 57, 63, 64, 65, 59, 60,
	70, 71, 38, 72, 73, 155, 87, 196, 79, 157,
	201, 56, 1, 58, 62, 66, 67, 164, 61, 68,
	69, 173, 174, 75, 55, 74, 193, 88, 156, 93,
	195, 170, 171, 167, 175, 178, 162, 176, 160, 172,
	169, 182, 161, 159, 185, 16, 14, 177, 16, 187,
	2, 184, 191, 16, 186, 0, 0, 0, 183, 192,
	0, 0, 0, 0, 0, 16, 189, 0, 0, 0,
	0, 200, 16, 207, 199, 16, 0, 0, 205, 208,
	0, 192, 0, 0, 204, 0, 0, 0, 206, 12,
	23, 24, 7, 0, 30, 37, 17, 18, 19, 21,
	22, 28, 0, 27, 136, 0, 8, 0, 9, 11,
	10, 0, 0, 0, 0, 0, 0, 29, 31, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	36, 0, 0, 25, 26, 20, 0, 0, 0, 0,
	0, 0, 0, 34, 35, 12, 23, 24, 7, 0,
	30, 37, 17, 18, 19, 21, 22, 28, 32, 27,
	33, 0, 8, 0, 9, 11, 10, 0, 0, 0,
	0, 0, 0, 29, 31, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 36, 0, 0, 25,
	26, 20, 0, 0, 0, 0, 0, 0, 0, 34,
	35, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 32, 0, 33, 98, 53, 54,
	57, 63, 64, 65, 59, 60, 70, 71, 0, 72,
	73, 0, 0, 0, 0, 99, 0, 56, 0, 58,
	62, 66, 67, 0, 61, 68, 69, 0, 0, 75,
	55, 74, 0, 0, 23, 24, 45, 135, 30, 37,
	17, 18, 19, 21, 22, 0, 0, 27, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 29, 31, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 36, 0, 0, 25, 26, 20,
	0, 0, 0, 0, 0, 0, 0, 34, 35, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 32, 0, 33, 98, 53, 54, 57, 63,
	64, 65, 59, 60, 70, 71, 0, 72, 73, 0,
	0, 0, 0, 99, 0, 56, 0, 58, 62, 66,
	67, 0, 61, 68, 69, 0, 0, 75, 55, 74,
	0, 165, 98, 53, 54, 57, 63, 64, 65, 59,
	60, 70, 71, 0, 72, 73, 0, 0, 0, 0,
	99, 0, 56, 0, 58, 62, 66, 67, 0, 61,
	68, 69, 0, 0, 75, 55, 74, 0, 151, 98,
	53, 54, 57, 63, 64, 65, 59, 60, 70, 71,
	0, 72, 73, 0, 0, 0, 0, 99, 0, 56,
	0, 58, 62, 66, 67, 0, 61, 68, 69, 0,
	0, 75, 55, 74, 0, 107, 23, 24, 45, 0,
	30, 37, 17, 18, 19, 21, 22, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 29, 31, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 36, 0, 0, 25,
	26, 20, 0, 0, 0, 0, 0, 0, 0, 34,
	35, 0, 0, 198, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 32, 0, 33, 98, 53, 54,
	57, 63, 64, 65, 59, 60, 70, 71, 0, 72,
	73, 0, 0, 0, 0, 99, 0, 56, 197, 58,
	62, 66, 67, 0, 61, 68, 69, 0, 0, 75,
	55, 74, 98, 53, 54, 57, 63, 64, 65, 59,
	60, 70, 71, 0, 72, 73, 0, 0, 0, 0,
	99, 104, 56, 0, 58, 62, 66, 67, 0, 61,
	68, 69, 0, 0, 75, 55, 74, 0, 98, 53,
	54, 57, 63, 64, 65, 59, 60, 70, 71, 0,
	72, 73, 0, 0, 0, 0, 99, 0, 56, 0,
	58, 62, 66, 67, 0, 61, 68, 69, 163, 0,
	75, 55, 74, 98, 53, 54, 57, 63, 64, 65,
	59, 60, 70, 71, 0, 72, 73, 0, 0, 0,
	0, 99, 0, 56, 0, 58, 62, 66, 67, 0,
	61, 68, 69, 166, 0, 75, 55, 74, 0, 0,
	0, 98, 53, 54, 57, 63, 64, 65, 59, 60,
	70, 71, 0, 72, 73, 0, 0, 0, 0, 99,
	0, 56, 0, 58, 62, 66, 67, 0, 61, 68,
	69, 163, 0, 75, 55, 74, 52, 53, 54, 57,
	63, 64, 65, 59, 60, 70, 71, 0, 72, 73,
	0, 0, 0, 0, 51, 0, 56, 146, 58, 62,
	66, 67, 0, 61, 68, 69, 0, 0, 75, 55,
	74, 98, 53, 54, 57, 63, 64, 65, 59, 60,
	70, 71, 0, 72, 73, 0, 0, 0, 0, 99,
	0, 56, 0, 58, 62, 66, 67, 0, 61, 68,
	69, 106, 0, 75, 55, 74, 0, 0, 98, 53,
	54, 57, 63, 64, 65, 59, 60, 70, 71, 0,
	72, 73, 0, 0, 0, 0, 99, 0, 56, 97,
	58, 62, 66, 67, 0, 61, 68, 69, 0, 0,
	75, 55, 74, 98, 53, 54, 57, 63, 64, 65,
	59, 60, 70, 71, 0, 72, 73, 0, 0, 0,
	0, 99, 0, 56, 0, 58, 62, 66, 67, 0,
	61, 68, 69, 0, 0, 75, 55, 74, 98, 53,
	54, 57, 63, 64, 65, 59, 60, 70, 71, 0,
	72, 73, 0, 0, 0, 0, 99, 0, 56, 0,
	58, 62, 66, 67, 0, 61, 68, 69, 0, 0,
	75, 55, 74, 98, 53, 54, 57, 63, 64, 65,
	59, 60, 70, 71, 0, 72, 73, 0, 0, 0,
	0, 105, 0, 56, 0, 58, 62, 66, 67, 0,
	61, 68, 69, 0, 0, 75, 55, 74, 52, 53,
	54, 57, 63, 64, 65, 59, 60, 70, 71, 0,
	72, 73, 0, 0, 0, 0, 51, 0, 56, 0,
	58, 62, 66, 67, 0, 61, 68, 69, 0, 0,
	75, 55, 74, 98, 53, 54, 57, 63, 64, 65,
	0, 0, 70, 71, 0, 72, 73, 23, 24, 45,
	0, 30, 37, 56, 0, 58, 62, 66, 67, 0,
	61, 68, 69, 0, 0, 75, 55, 74, 23, 24,
	45, 0, 30, 37, 29, 31, 23, 24, 45, 0,
	30, 37, 0, 0, 0, 0, 0, 36, 0, 0,
	25, 26, 0, 0, 0, 29, 31, 0, 0, 0,
	34, 35, 0, 29, 31, 0, 0, 0, 36, 0,
	0, 25, 26, 0, 0, 32, 36, 33, 0, 25,
	26, 34, 158, 0, 0, 0, 0, 0, 0, 34,
	90, 0, 0, 0, 0, 0, 32, 0, 33, 0,
	0, 0, 0, 0, 32, 0, 33, 63, 64, 65,
	0, 0, 70, 71, 0, 72, 73, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 62, 66, 67, 0,
	61, 68, 69, 0, 0, 75, 55, 74, 0, 152,
	63, 64, 65, 0, 0, 70, 71, 0, 72, 73,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 62,
	66, 67, 0, 61, 68, 69, 0, 0, 75, 55,
	74, 63, 64, 65, 0, 0, 70, 71, 0, 72,
	73, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 66, 67, 0, 61, 68, 69, 0, 0, 75,
	55, 74,
}
var yyPact = []int{

	271, -1000, 124, 271, -1000, -1000, 77, 1072, 69, 1072,
	1072, 1072, 1072, -1000, -1000, 994, 7, 1072, -19, -21,
	-23, -25, -26, 1072, -1000, -1000, -1000, 271, 127, -1000,
	-1000, -1000, 1101, 1072, 1072, 1072, 1072, 1072, -1000, -1000,
	-38, -1000, 889, 75, -1000, 1072, 57, 674, 959, 854,
	495, 1072, 1072, 1072, 1072, 1072, 1072, 1072, 1072, 1072,
	1072, 1072, 1072, 1072, 1072, 1072, 1072, 1072, 1072, 1072,
	1072, 1072, 1072, 1072, 1072, 83, -1000, -40, -10, 924,
	-1000, -1000, -1000, -1000, -1000, 313, 215, -54, -32, 924,
	1072, -1000, -1000, -1000, -27, -60, -1000, 379, 1072, 1072,
	-1000, 817, 1072, 69, -1000, 1072, 71, -1000, 458, 1139,
	1172, 1172, 25, 1172, 1172, 1172, 1029, 1029, 50, 1203,
	-27, 50, 50, 50, 50, -27, -27, -27, -27, -27,
	-27, -48, -1000, -1000, 1072, -1000, -1000, 1072, 1093, -1000,
	-1000, -1000, -1000, 782, 1172, 924, 1072, 421, -1000, 747,
	-1000, -1000, -1000, -1000, -1000, 924, -24, 924, 1072, -1000,
	133, -1000, 123, 1072, 709, -1000, 1072, 74, -43, -1000,
	1072, 379, -1000, 1072, 561, 87, 674, 66, -1000, 379,
	128, -1000, 638, -1000, -1000, 603, -1000, 69, -1000, -1000,
	-1000, 561, -1000, 62, -1000, -1000, -1000, 69, 561, -1000,
	-1000, 379, 128, -1000, -1000, -1000, -1000, -1000, -1000,
}
var yyPgo = []int{

	0, 83, 182, 180, 15, 9, 16, 1, 177, 176,
	173, 172, 170, 169, 168, 166, 165, 163, 81, 20,
	157, 156, 0, 3, 6, 22, 142, 2, 140,
}
var yyR1 = []int{

	0, 26, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	4, 4, 9, 27, 27, 6, 7, 2, 2, 1,
	1, 3, 3, 10, 11, 14, 14, 15, 15, 12,
	12, 12, 13, 13, 17, 17, 16, 16, 28, 28,
	8, 8, 8, 18, 18, 19, 19, 21, 21, 24,
	24, 20, 20, 20, 20, 22, 22, 22, 22, 22,
	22, 22, 22, 22, 22, 22, 22, 22, 22, 22,
	22, 22, 22, 22, 22, 22, 22, 22, 22, 22,
	22, 22, 22, 22, 22, 22, 22, 22, 22, 22,
	22, 22, 23, 25, 25, 25, 25,
}
var yyR2 = []int{

	0, 2, 4, 4, 2, 3, 2, 2, 2, 2,
	2, 1, 3, 5, 5, 5, 4, 8, 7, 3,
	1, 1, 6, 0, 1, 3, 1, 1, 2, 1,
	2, 0, 1, 2, 2, 0, 5, 0, 5, 0,
	2, 2, 0, 2, 0, 2, 4, 4, 0, 1,
	0, 2, 2, 0, 1, 1, 3, 1, 3, 1,
	1, 1, 1, 3, 3, 1, 1, 2, 1, 2,
	1, 3, 3, 3, 3, 4, 3, 3, 3, 6,
	2, 3, 3, 3, 3, 3, 3, 3, 2, 2,
	3, 3, 3, 3, 3, 3, 2, 3, 3, 4,
	3, 3, 3, 3, 1, 1, 1,
}
var yyChk = []int{

	-1000, -26, -3, -1, -4, -5, -25, 7, 21, 23,
	25, 24, 4, -6, -9, -22, -23, 11, 12, 13,
	50, 14, 15, 5, 6, 48, 49, 18, 16, 32,
	9, 33, 73, 75, 58, 59, 45, 10, 8, -4,
	9, 33, -22, -25, -23, 7, -6, -22, -22, -22,
	-22, 52, 34, 35, 36, 67, 54, 37, 56, 41,
	42, 61, 57, 38, 39, 40, 58, 59, 62, 63,
	43, 44, 46, 47, 68, 66, 70, -18, -19, -22,
	70, 70, 70, 70, 70, -22, -1, 9, -20, -22,
	59, -22, -22, -22, -22, -18, 70, 20, 34, 52,
	9, -22, 22, -27, 17, 52, 27, 70, -22, -22,
	-22, -22, -19, -22, -22, -22, -22, -22, -22, -22,
	-22, -22, -22, -22, -22, -22, -22, -22, -22, -22,
	-22, -19, 9, 70, 53, 74, 19, 71, 53, 74,
	74, -6, -5, -22, -22, -22, 20, -22, -6, -22,
	18, 70, 70, 55, 76, -22, -19, -22, 59, -10,
	-14, -11, -15, 29, -22, 70, 26, -17, 72, -12,
	28, 29, -13, 28, 29, -22, -22, -8, -16, 30,
	31, 70, -22, -6, -5, -22, -5, -27, 19, -6,
	-7, -2, -5, -21, -24, 32, 9, 20, 20, -6,
	-5, -28, 53, 20, -6, -5, -6, -7, -24,
}
var yyDef = []int{

	31, -2, 0, 32, 29, 11, 0, 0, 0, 0,
	0, 0, 0, 20, 21, 0, 70, 53, 0, 0,
	0, 0, 0, 0, 104, 105, 106, 0, 0, 65,
	66, 68, 0, 0, 0, 0, 0, 53, 1, 30,
	67, 69, 0, 0, 70, 0, 0, 23, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 4, 0, 54, 55,
	6, 7, 8, 9, 10, 0, 0, 0, 0, 61,
	62, 80, 88, 89, 96, 0, 12, 0, 0, 0,
	67, 0, 0, 0, 24, 0, 0, 19, 101, 72,
	73, 74, 0, 76, 77, 78, 81, 82, 83, 84,
	85, 86, 87, 90, 91, 92, 93, 94, 95, 97,
	98, 0, 100, 5, 0, 103, 25, 0, 0, 71,
	102, 35, 37, 0, 72, 101, 0, 0, 16, 0,
	44, 2, 3, 75, 99, 56, 0, 63, 64, 13,
	39, 14, 42, 0, 0, 15, 0, 50, 0, 33,
	0, 0, 34, 0, 0, 79, 23, 0, 45, 0,
	0, 22, 0, 40, 41, 0, 43, 0, 18, 51,
	52, 26, 27, 48, 57, 59, 60, 0, 0, 17,
	28, 0, 0, 49, 36, 38, 46, 47, 58,
}
var yyTok1 = []int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 75, 3, 3, 3, 64, 65, 3,
	73, 74, 62, 58, 53, 59, 66, 63, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 57, 70,
	67, 52, 55, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 68, 3, 76, 61, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 71, 60, 72,
}
var yyTok2 = []int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
	22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 40, 41,
	42, 43, 44, 45, 46, 47, 48, 49, 50, 51,
	54, 56, 69,
}
var yyTok3 = []int{
	0,
}

//line yaccpar:1

/*	parser for yacc output	*/

var yyDebug = 0

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

const yyFlag = -1000

func yyTokname(c int) string {
	// 4 is TOKSTART above
	if c >= 4 && c-4 < len(yyToknames) {
		if yyToknames[c-4] != "" {
			return yyToknames[c-4]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yylex1(lex yyLexer, lval *yySymType) int {
	c := 0
	char := lex.Lex(lval)
	if char <= 0 {
		c = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		c = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			c = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		c = yyTok3[i+0]
		if c == char {
			c = yyTok3[i+1]
			goto out
		}
	}

out:
	if c == 0 {
		c = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(c), uint(char))
	}
	return c
}

func yyParse(yylex yyLexer) int {
	var yyn int
	var yylval yySymType
	var yyVAL yySymType
	yyS := make([]yySymType, yyMaxDepth)

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yychar := -1
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yychar), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yychar < 0 {
		yychar = yylex1(yylex, &yylval)
	}
	yyn += yychar
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yychar { /* valid shift */
		yychar = -1
		yyVAL = yylval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yychar < 0 {
			yychar = yylex1(yylex, &yylval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yychar {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error("syntax error")
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yychar))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yychar))
			}
			if yychar == yyEofCode {
				goto ret1
			}
			yychar = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= yyR2[yyn]
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 1:
		//line pseudo.y:100
		{
			yylex.(*Lexer).prog = yyS[yypt-1].stmts
			return 0
		}
	case 2:
		//line pseudo.y:107
		{
			yyVAL.stmt = &Stmt{Op: Assign, X: yyS[yypt-3].expr, Y: yyS[yypt-1].expr}
		}
	case 3:
		//line pseudo.y:111
		{
			yyVAL.stmt = &Stmt{Op: Assign, X: yyS[yypt-3].expr, Y: yyS[yypt-1].expr}
		}
	case 4:
		//line pseudo.y:115
		{
			yyVAL.stmt = &Stmt{Op: StmtExpr, X: yyS[yypt-1].expr}
		}
	case 5:
		//line pseudo.y:119
		{
			yyVAL.stmt = &Stmt{Op: Return, List: yyS[yypt-1].exprs}
		}
	case 6:
		//line pseudo.y:123
		{
			yyVAL.stmt = &Stmt{Op: Undefined}
		}
	case 7:
		//line pseudo.y:127
		{
			yyVAL.stmt = &Stmt{Op: Unpredictable}
		}
	case 8:
		//line pseudo.y:131
		{
			yyVAL.stmt = &Stmt{Op: See, Text: yyS[yypt-1].str}
		}
	case 9:
		//line pseudo.y:135
		{
			yyVAL.stmt = &Stmt{Op: ImplDefined}
		}
	case 10:
		//line pseudo.y:139
		{
			yyVAL.stmt = &Stmt{Op: SubarchDefined}
		}
	case 11:
		yyVAL.stmt = yyS[yypt-0].stmt
	case 12:
		//line pseudo.y:146
		{
			yyVAL.stmt = &Stmt{Op: StmtExpr, X: &Expr{Op: Decl, Type: yyS[yypt-2].typ, Text: yyS[yypt-1].str}}
		}
	case 13:
		//line pseudo.y:150
		{
			yyVAL.stmt = &Stmt{Op: If, X: yyS[yypt-3].expr, Body: yyS[yypt-1].stmt, ElseIf: yyS[yypt-0].elseifs, Else: yyS[yypt-0].stmt}
		}
	case 14:
		//line pseudo.y:154
		{
			yyVAL.stmt = &Stmt{Op: If, X: yyS[yypt-3].expr, Body: yyS[yypt-1].stmt, ElseIf: yyS[yypt-0].elseifs, Else: yyS[yypt-0].stmt}
		}
	case 15:
		//line pseudo.y:158
		{
			yyVAL.stmt = &Stmt{Op: Repeat, Body: yyS[yypt-3].stmt, X: yyS[yypt-1].expr}
		}
	case 16:
		//line pseudo.y:162
		{
			yyVAL.stmt = &Stmt{Op: While, X: yyS[yypt-2].expr, Body: yyS[yypt-0].stmt}
		}
	case 17:
		//line pseudo.y:166
		{
			yyVAL.stmt = &Stmt{Op: For, X: yyS[yypt-6].expr, Y: yyS[yypt-4].expr, Z: yyS[yypt-2].expr, Body: yyS[yypt-0].stmt}
		}
	case 18:
		//line pseudo.y:170
		{
			yyVAL.stmt = &Stmt{Op: Case, X: yyS[yypt-5].expr, When: yyS[yypt-2].whens, Else: yyS[yypt-1].stmt}
		}
	case 19:
		//line pseudo.y:174
		{
			yyVAL.stmt = &Stmt{Op: Assert, X: yyS[yypt-1].expr}
		}
	case 20:
		//line pseudo.y:178
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 21:
		yyVAL.stmt = yyS[yypt-0].stmt
	case 22:
		//line pseudo.y:185
		{
			yyVAL.stmt = &Stmt{Op: Enum, Text: yyS[yypt-4].str, List: yyS[yypt-2].exprs}
		}
	case 25:
		//line pseudo.y:194
		{
			yyVAL.stmt = &Stmt{Op: Block, Block: yyS[yypt-1].stmts}
		}
	case 26:
		//line pseudo.y:200
		{
			yyVAL.stmt = &Stmt{Op: Block, Block: yyS[yypt-0].stmts}
		}
	case 27:
		//line pseudo.y:206
		{
			yyVAL.stmts = []*Stmt{yyS[yypt-0].stmt}
		}
	case 28:
		//line pseudo.y:210
		{
			yyVAL.stmts = append(yyS[yypt-1].stmts, yyS[yypt-0].stmt)
		}
	case 29:
		//line pseudo.y:216
		{
			yyVAL.stmts = []*Stmt{yyS[yypt-0].stmt}
		}
	case 30:
		//line pseudo.y:220
		{
			yyVAL.stmts = append(yyS[yypt-1].stmts, yyS[yypt-0].stmt)
		}
	case 31:
		//line pseudo.y:225
		{
			yyVAL.stmts = nil
		}
	case 32:
		yyVAL.stmts = yyS[yypt-0].stmts
	case 33:
		//line pseudo.y:232
		{
			yyVAL.elseifs = yyS[yypt-1].elseifs
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 34:
		//line pseudo.y:239
		{
			yyVAL.elseifs = yyS[yypt-1].elseifs
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 35:
		//line pseudo.y:245
		{
			yyVAL.elseifs = nil
		}
	case 36:
		//line pseudo.y:249
		{
			yyVAL.elseifs = append(yyS[yypt-4].elseifs, &ElseIf{Cond: yyS[yypt-2].expr, Body: yyS[yypt-0].stmt})
		}
	case 37:
		//line pseudo.y:254
		{
			yyVAL.elseifs = nil
		}
	case 38:
		//line pseudo.y:258
		{
			yyVAL.elseifs = append(yyS[yypt-4].elseifs, &ElseIf{Cond: yyS[yypt-2].expr, Body: yyS[yypt-0].stmt})
		}
	case 39:
		//line pseudo.y:263
		{
			yyVAL.stmt = nil
		}
	case 40:
		//line pseudo.y:267
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 41:
		//line pseudo.y:271
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 42:
		//line pseudo.y:276
		{
			yyVAL.stmt = nil
		}
	case 43:
		//line pseudo.y:280
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 44:
		//line pseudo.y:285
		{
			yyVAL.whens = nil
		}
	case 45:
		//line pseudo.y:289
		{
			yyVAL.whens = append(yyS[yypt-1].whens, yyS[yypt-0].when)
		}
	case 46:
		//line pseudo.y:295
		{
			yyVAL.when = &When{Cond: yyS[yypt-2].exprs, Body: yyS[yypt-0].stmt}
		}
	case 47:
		//line pseudo.y:299
		{
			yyVAL.when = &When{Cond: yyS[yypt-2].exprs, Body: yyS[yypt-0].stmt}
		}
	case 50:
		//line pseudo.y:307
		{
			yyVAL.stmt = nil
		}
	case 51:
		//line pseudo.y:311
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 52:
		//line pseudo.y:315
		{
			yyVAL.stmt = yyS[yypt-0].stmt
		}
	case 53:
		//line pseudo.y:320
		{
			yyVAL.exprs = nil
		}
	case 54:
		yyVAL.exprs = yyS[yypt-0].exprs
	case 55:
		//line pseudo.y:327
		{
			yyVAL.exprs = []*Expr{yyS[yypt-0].expr}
		}
	case 56:
		//line pseudo.y:331
		{
			yyVAL.exprs = append(yyS[yypt-2].exprs, yyS[yypt-0].expr)
		}
	case 57:
		//line pseudo.y:337
		{
			yyVAL.exprs = []*Expr{yyS[yypt-0].expr}
		}
	case 58:
		//line pseudo.y:341
		{
			yyVAL.exprs = append(yyS[yypt-2].exprs, yyS[yypt-0].expr)
		}
	case 59:
		//line pseudo.y:347
		{
			yyVAL.expr = &Expr{Op: Const, Text: yyS[yypt-0].str}
		}
	case 60:
		//line pseudo.y:351
		{
			yyVAL.expr = &Expr{Op: Name, Text: yyS[yypt-0].str}
		}
	case 61:
		//line pseudo.y:357
		{
			yyVAL.exprs = []*Expr{yyS[yypt-0].expr}
		}
	case 62:
		//line pseudo.y:361
		{
			yyVAL.exprs = []*Expr{&Expr{Op: Blank}}
		}
	case 63:
		//line pseudo.y:365
		{
			yyVAL.exprs = append(yyS[yypt-2].exprs, yyS[yypt-0].expr)
		}
	case 64:
		//line pseudo.y:369
		{
			yyVAL.exprs = append(yyS[yypt-2].exprs, &Expr{Op: Blank})
		}
	case 65:
		//line pseudo.y:375
		{
			yyVAL.expr = &Expr{Op: Const, Text: yyS[yypt-0].str}
		}
	case 66:
		//line pseudo.y:379
		{
			yyVAL.expr = &Expr{Op: Name, Text: yyS[yypt-0].str}
		}
	case 67:
		//line pseudo.y:383
		{
			yyVAL.expr = &Expr{Op: Decl, Type: yyS[yypt-1].typ, Text: yyS[yypt-0].str}
		}
	case 68:
		//line pseudo.y:387
		{
			yyVAL.expr = &Expr{Op: Unknown}
		}
	case 69:
		//line pseudo.y:391
		{
			yyVAL.expr = &Expr{Op: Unknown, Type: yyS[yypt-1].typ}
		}
	case 70:
		yyVAL.expr = yyS[yypt-0].expr
	case 71:
		//line pseudo.y:396
		{
			yyVAL.expr = &Expr{Op: ExprTuple, List: yyS[yypt-1].exprs}
		}
	case 72:
		//line pseudo.y:400
		{
			yyVAL.expr = &Expr{Op: Eq, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 73:
		//line pseudo.y:404
		{
			yyVAL.expr = &Expr{Op: NotEq, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 74:
		//line pseudo.y:408
		{
			yyVAL.expr = &Expr{Op: LtEq, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 75:
		//line pseudo.y:412
		{
			yyVAL.expr = &Expr{Op: BitIndex, X: yyS[yypt-3].expr, List: yyS[yypt-1].exprs}
		}
	case 76:
		//line pseudo.y:416
		{
			yyVAL.expr = &Expr{Op: Lt, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 77:
		//line pseudo.y:420
		{
			yyVAL.expr = &Expr{Op: GtEq, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 78:
		//line pseudo.y:424
		{
			yyVAL.expr = &Expr{Op: Gt, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 79:
		//line pseudo.y:428
		{
			yyVAL.expr = &Expr{Op: IfElse, X: yyS[yypt-4].expr, Y: yyS[yypt-2].expr, Z: yyS[yypt-0].expr}
		}
	case 80:
		//line pseudo.y:432
		{
			yyVAL.expr = &Expr{Op: Not, X: yyS[yypt-0].expr}
		}
	case 81:
		//line pseudo.y:436
		{
			yyVAL.expr = &Expr{Op: AndAnd, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 82:
		//line pseudo.y:440
		{
			yyVAL.expr = &Expr{Op: OrOr, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 83:
		//line pseudo.y:444
		{
			yyVAL.expr = &Expr{Op: Eor, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 84:
		//line pseudo.y:448
		{
			yyVAL.expr = &Expr{Op: Colon, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 85:
		//line pseudo.y:452
		{
			yyVAL.expr = &Expr{Op: BigAND, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 86:
		//line pseudo.y:456
		{
			yyVAL.expr = &Expr{Op: BigOR, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 87:
		//line pseudo.y:460
		{
			yyVAL.expr = &Expr{Op: BigEOR, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 88:
		//line pseudo.y:464
		{
			yyVAL.expr = &Expr{Op: Plus, X: yyS[yypt-0].expr}
		}
	case 89:
		//line pseudo.y:468
		{
			yyVAL.expr = &Expr{Op: Minus, X: yyS[yypt-0].expr}
		}
	case 90:
		//line pseudo.y:472
		{
			yyVAL.expr = &Expr{Op: Add, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 91:
		//line pseudo.y:476
		{
			yyVAL.expr = &Expr{Op: Sub, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 92:
		//line pseudo.y:480
		{
			yyVAL.expr = &Expr{Op: Mul, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 93:
		//line pseudo.y:484
		{
			yyVAL.expr = &Expr{Op: Div, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 94:
		//line pseudo.y:488
		{
			yyVAL.expr = &Expr{Op: BigDIV, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 95:
		//line pseudo.y:492
		{
			yyVAL.expr = &Expr{Op: BigMOD, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 96:
		//line pseudo.y:496
		{
			yyVAL.expr = &Expr{Op: TwoPow, X: yyS[yypt-0].expr}
		}
	case 97:
		//line pseudo.y:500
		{
			yyVAL.expr = &Expr{Op: Lsh, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 98:
		//line pseudo.y:504
		{
			yyVAL.expr = &Expr{Op: Rsh, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 99:
		//line pseudo.y:508
		{
			yyVAL.expr = &Expr{Op: Index, X: yyS[yypt-3].expr, List: yyS[yypt-1].exprs}
		}
	case 100:
		//line pseudo.y:512
		{
			yyVAL.expr = &Expr{Op: Dot, X: yyS[yypt-2].expr, Text: yyS[yypt-0].str}
		}
	case 101:
		//line pseudo.y:516
		{
			yyVAL.expr = &Expr{Op: Eq, X: yyS[yypt-2].expr, Y: yyS[yypt-0].expr}
		}
	case 102:
		//line pseudo.y:522
		{
			yyVAL.expr = &Expr{Op: Call, Text: yyS[yypt-2].str, List: yyS[yypt-1].exprs}
		}
	case 103:
		//line pseudo.y:528
		{
			yyVAL.typ = &Type{Op: BitType, NX: yyS[yypt-1].expr}
		}
	case 104:
		//line pseudo.y:532
		{
			yyVAL.typ = &Type{Op: BitType, N: 1}
		}
	case 105:
		//line pseudo.y:536
		{
			yyVAL.typ = &Type{Op: IntegerType}
		}
	case 106:
		//line pseudo.y:540
		{
			yyVAL.typ = &Type{Op: BoolType}
		}
	}
	goto yystack /* stack new state and value */
}
