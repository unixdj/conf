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
	errIllOpt = errors.New("illegal option")
	errNoArg  = errors.New("option requires an argument")
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
	if e.Value != "" {
		return e.Err.Error() + " -- " + e.Value
	}
	return e.Err.Error() + " -- " + string(e.Flag)
}

// newError creates FlagError from f, v and e
func newError(f rune, v string, e error) *FlagError {
	return &FlagError{f, v, e}
}

func getFlag(flag rune, vars []Var) *Var {
	for i := range vars {
		if vars[i].Flag == flag {
			return &vars[i]
		}
	}
	return nil
}

func GetOpt(vars []Var) error {
	Args = make([]string, len(os.Args)-1)
	copy(Args, os.Args[1:])
	for len(Args) > 0 && len(Args[0]) > 1 && Args[0][0] == '-' {
		if Args[0] == "--" {
			Args = Args[1:]
			break
		}
		var this, p string
		this, Args = Args[0][1:], Args[1:]
		for len(this) > 0 {
			flag, size := utf8.DecodeRuneInString(this)
			this = this[size:]
			if flag == utf8.RuneError {
				return newError(flag, "", errSyntax)
			}
			v := getFlag(flag, vars)
			if v == nil {
				return newError(flag, "", errIllOpt)
			}
			switch {
			case v.Bare:
				p = "true"
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
		}
	}
	return nil
}
