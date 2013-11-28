// Copyright 2012 Vadim Vygonets
// This program is free software. It comes without any warranty, to
// the extent permitted by applicable law. You can redistribute it
// and/or modify it under the terms of the Do What The Fuck You Want
// To Public License, Version 2, as published by Sam Hocevar. See
// the LICENSE file or http://sam.zoy.org/wtfpl/ for more details.

package conf

import (
	"errors"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	errIllOpt  = errors.New("illegal option")
	errNoArg   = errors.New("option requires an argument")
	errEndJunk = errors.New("junk at end of option")
)

var Args []string

// FlagError represents the error.
type FlagError struct {
	Flag  rune   // flag
	Value string // value
	Err   error  // error
}

// Error prints FlagError as follows, if Value is not empty:
//     Err -- Value
// otherwise:
//     Err -- Flag
func (e *FlagError) Error() string {
	if e.Value != "" && e.Value != "true" {
		return e.Err.Error() + " -- " + e.Value
	}
	return e.Err.Error() + " -- " + string(e.Flag)
}

// newError creates FlagError from f, v and e
func newError(f rune, v string, e error) *FlagError {
	return &FlagError{f, v, e}
}

// flavour
const (
	short = iota
	xLong
	gnuLong
)

const (
	shortFlag = iota
	longFlag
	gnuLongFlag
	falseFlag
	endArg
	endArgSkip
)

func nextArg(arg string, flavour int) (int, string) {
	if len(arg) <= 1 {
		return endArg, ""
	}
	switch arg[0] {
	case '-':
		if arg[1] == '-' {
			if len(arg) == 2 {
				return endArgSkip, ""
			}
			if flavour == gnuLong {
				return gnuLongFlag, arg[2:]
			}
		}
		if flavour == xLong {
			return longFlag, arg[1:]
		}
		return shortFlag, arg[1:]
	case '+':
		if flavour == xLong {
			return falseFlag, arg[1:]
		}
	}
	return endArg, ""
}

func nextFlag(this string, kind int) (rune, string, string) {
	switch kind {
	case shortFlag:
		flag, size := utf8.DecodeRuneInString(this)
		return flag, "", this[size:]
	case gnuLongFlag:
		if pos := strings.Index(this, "="); pos != -1 {
			return '=', this[:pos], this[pos+1:]
		}
	}
	// longFlag or bare gnuLongFlag
	return 0, this, ""
}

func findFlag(flag rune, long string, kind int, vars []Var) *Var {
	var eq func(i int) bool
	if kind == shortFlag {
		eq = func(i int) bool { return vars[i].Flag == flag }
	} else {
		eq = func(i int) bool { return vars[i].Name == long }
	}
	for i := range vars {
		if eq(i) {
			return &vars[i]
		}
	}
	return nil
}

func doGetOpt(vars []Var, flavour int) error {
	Args = make([]string, len(os.Args)-1)
	copy(Args, os.Args[1:])
	for len(Args) > 0 {
		kind, this := nextArg(Args[0], flavour)
		if kind == endArg {
			break
		}
		Args = Args[1:]
		if kind == endArgSkip {
			break
		}
		for len(this) > 0 {
			var (
				flag    rune
				long, p string
			)
			flag, long, this = nextFlag(this, kind)
			if flag == utf8.RuneError {
				return newError(flag, long, errSyntax)
			}
			v := findFlag(flag, long, kind, vars)
			if v == nil {
				return newError(flag, long, errIllOpt)
			}
			switch {
			case kind == falseFlag:
				if v.Kind != NoArg {
					return newError(flag, long, errIllOpt)
				}
				p = "false"
			case v.Kind == NoArg:
				if kind == gnuLongFlag && flag == '=' {
					return newError(0, this, errEndJunk)
				}
				p = "true"
			case v.Kind == LineArg:
				if this != "" {
					// XXX
					return newError(0, this, errEndJunk)
				}
			case this != "":
				p, this = this, ""
			case kind == gnuLongFlag && flag == '=':
				// empty parameter
			case len(Args) != 0:
				p, Args = Args[0], Args[1:]
			default:
				return newError(flag, "", errNoArg)
			}
			if err := v.Val.Set(p); err != nil {
				return newError(flag, p, err)
			}
			v.flagSet = true
			if v.Kind == LineArg {
				break
			}
		}
	}
	return nil
}

/*
GetOpt parses command line flags in the traditional Unix
manner, stopping at the first unrecognized argument, without
glibc-style flags-after-parameters bullshit.  Special
handling of "-W" flags and getsubopt() are not supported.
The unparsed command line arguments are kept in the Args array.

GetOpt ignores the Name field of vars, only parsing short flags.

Command line arguments parsed by GetOpt begin with a dash followed
by one or more characters.  The special argument "--" (double dash)
stops command line processing, keeping subsequent arguments in Args.
Command line processing also stops at the first non-flag argument
("-" (single dash) or one that doesn't begin with a dash), or after
a LineArg flag, as described below.

After skipping the initial dash, each argument is parsed for flags
as follows.

vars is searched for the Var whose Flag is equal to the fist character
of the remaining argument.
If none is found, an error is returned.  Otherwise, its Value.Set is
called with a string parameter, depending on the value of its Kind member.

For a Var whose Kind is NoArg, the parameter is "true" (for
compatibility with BoolValue).  After encountering a NoArg flag,
flag processing is restarted at the next character.

For HasArg, if the rest of the argument is not empty, it becomes
the parameter.  Otherwise the next argument is used, and
non-existence thereof is treated as an error.  Command line
argument processing is restarted at the next argument.

For LineArg, the parameter is an empty string, and the rest of
the argument must be empty.  The Set function is expected to
peruse Args.  Command line processing is stopped after a LineArg.
*/
func GetOpt(vars []Var) error {
	return doGetOpt(vars, short)
}

/*
GetOptLong parses command line options in GNU style, with long
options prepended by "--".

GetOptLong functions like GetOpt, except for long arguments
starting with "--" (two dashes) followed by one or more
character.

Long arguments can take the form "--name=value" or "--name".
vars is searched for a Var whose Name is equal to the "name"
part of the argument.
The first form is only allowed for vars whose Kind is HasArg.
HasArg vars of the second form use the next argument as the
value (i.e., parameter to Value.Set).  NoArg and LineArg are
treated as in GetOpt.
*/
func GetOptLong(vars []Var) error {
	return doGetOpt(vars, gnuLong)
}

/*
GetOptLongOnly parses command line options in X11 manner, with
long options prepended by "-" or "+", the latter to reset a
boolean option.  It ignores the Flag field of vars, treating all
flags as long.

Command line arguments parsed by GetOpt begin with a dash or a plus,
followed by one or more characters.  The special argument "--"
(double dash) stops command line processing, keeping subsequent
arguments in Args.
Command line processing also stops at the first non-flag argument
("-" (single dash), "+" or one that doesn't begin with a dash or
a plus), or after a LineArg flag, as described below.

vars is searched for the Var whose Name is equal to the part of
the argument after the initial "-" or "+".
If none is found, an error is returned.  Otherwise, its Value.Set
is called with a string parameter.

If the argument starts with "+", the Kind of the Var must be
NoArg, in which case the parameter is "false".  For one that
starts with "-", it depends on the Kind of the Var.

For a Var whose Kind is NoArg, the parameter is "true" (for
compatibility with BoolValue).
For HasArg, the next argument is used as the parameter.
For LineArg, the parameter is an empty string, and the
command line processing stops.
*/
func GetOptLongOnly(vars []Var) error {
	return doGetOpt(vars, xLong)
}
