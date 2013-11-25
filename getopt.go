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
	Short = iota
	XLong
	GnuLong
)

const (
	shortFlag = iota
	longFlag
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
			if flavour == GnuLong {
				return longFlag, arg[2:]
			}
		}
		if flavour == XLong {
			return longFlag, arg[1:]
		}
		return shortFlag, arg[1:]
	case '+':
		if flavour == XLong {
			return falseFlag, arg[1:]
		}
	}
	return endArg, ""
}

func nextFlag(this string, kind int) (rune, string, string) {
	if kind != shortFlag {
		return 0, this, ""
	}
	flag, size := utf8.DecodeRuneInString(this)
	return flag, "", this[size:]
}

func getFlag(flag rune, long string, kind int, vars []Var) *Var {
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
			v := getFlag(flag, long, kind, vars)
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
				p = "true"
			case v.Kind == LineArg:
				if this != "" {
					// XXX
					return newError(0, this, errEndJunk)
				}
			case this != "":
				p, this = this, ""
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

// GetOpt parses command line options in the traditional Unix
// manner, stopping at the first unrecognized argument, without
// the GNU/Linux flags-after-parameters bullshit.  Special
// handling of "-W" flags and getsubopt() are not supported.
//
// GetOpt ignores the Name field of vars.
func GetOpt(vars []Var) error {
	return doGetOpt(vars, Short)
}

// GetOptLong parses command line options in GNU style, with long
// options prepended by "--".
func GetOptLong(vars []Var) error {
	return doGetOpt(vars, GnuLong)
}

// GetOptLongOnly parses command line options in X11 manner, with
// long options prepended by "-" or "+", the latter to reset a
// boolean option.
//
// GetOptLongOnly ignores the Flag field of vars, treating all
// flags as long.
func GetOptLongOnly(vars []Var) error {
	return doGetOpt(vars, XLong)
}
