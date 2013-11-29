// Copyright 2012 Vadim Vygonets
// This program is free software. It comes without any warranty, to
// the extent permitted by applicable law. You can redistribute it
// and/or modify it under the terms of the Do What The Fuck You Want
// To Public License, Version 2, as published by Sam Hocevar. See
// the LICENSE file or http://sam.zoy.org/wtfpl/ for more details.

package conf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Value is the interface to the value pointed to by Var.
// Built-in Values also implement String() for compatibility with flag.Value.
// A usage example is in example/example.go.
type Value interface {
	// Set receives a string value after it's been unquoted.
	// The error it returns, if not nil, gets wrapped in ParseError.
	Set(string) error
}

// StringValue represents a configuration variable's string value.
type StringValue string

func (v *StringValue) Set(s string) error {
	*v = StringValue(s)
	return nil
}

func (v *StringValue) String() string { return string(*v) }

// BoolValue represents a configuration variable's boolean value.
// Syntax: 0/false/f/off/no/n/disabled 1/true/t/on/yes/y/enabled
// (case insensitive).
type BoolValue bool

func strInList(s string, l []string) bool {
	for _, v := range l {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func (v *BoolValue) Set(s string) error {
	switch {
	case strInList(s, []string{"0", "false", "f", "off", "no", "n", "disabled"}):
		*v = false
	case strInList(s, []string{"1", "true", "t", "on", "yes", "y", "enabled"}):
		*v = true
	default:
		return errSyntax
	}
	return nil
}

func (v *BoolValue) String() string { return strconv.FormatBool(bool(*v)) }

// Int64Value represents a configuration variable's int64 value.
// Numeric values can be given as decimal, octal or hexadecimal
// in the usual C/Go manner (255 == 0377 == 0xff).
type Int64Value int64

func (v *Int64Value) Set(s string) error {
	u, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		// strip fluff from strconf.ParseInt
		return err.(*strconv.NumError).Err
	}
	*v = Int64Value(u)
	return nil
}

func (v *Int64Value) String() string { return strconv.FormatInt(int64(*v), 10) }

// Uint64Value represents a configuration variable's uint64 value.
// Numeric values can be given as decimal, octal or hexadecimal
// in the usual C/Go manner (255 == 0377 == 0xff).
type Uint64Value uint64

func (v *Uint64Value) Set(s string) error {
	u, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		// strip fluff from strconf.ParseUint
		return err.(*strconv.NumError).Err
	}
	*v = Uint64Value(u)
	return nil
}

func (v *Uint64Value) String() string { return strconv.FormatUint(uint64(*v), 10) }

type funcValue struct {
	f func(string) error
}

func (v funcValue) Set(s string) error {
	return v.f(s)
}

// FuncValue returns a Value whose Set method is f.
func FuncValue(f func(string) error) Value {
	return funcValue{f}
}

const (
	HasArg  = iota // flag requires arguments
	NoArg          // boolean flag with no arguments
	LineArg        // flag ends processing
)

// Var describes a configuration variable / command line flag
// and has pointers to corresponding (Go) variables.
// Slice of Var is used for calling Parse() and GetOpt{,Long{,Only}}().
type Var struct {
	Flag     rune   // short option
	Name     string // name of configuration variable / long option
	Val      Value  // Value to set
	Kind     int    // HasArg / NoArg / LineArg
	Required bool   // variable is required to be set in conf file
	set      bool   // has been set from conf file
	flagSet  bool   // has been set from command line
}

type parser struct {
	r     *bufio.Reader
	file  string
	line  int
	ident string
	value string
	vars  []Var
}

var (
	errSyntax      = errors.New("syntax error")
	errLineTooLong = errors.New("line too long")
	errReqNotSet   = errors.New("required but not set")
	errAlreadyDef  = errors.New("already defined")
	errUnknownVar  = errors.New("unknown variable")
)

// ParseError represents a configuration file parsing error.
type ParseError struct {
	File  string // filename or "stdin"
	Line  int    // line number or 0
	Ident string // identifier or ""
	Value string // value as appears in input, possibly quoted; or ""
	Err   error  // error
}

// Error prints ParseError as follows:
//     File:[Line:][ Ident:] Err
// Value never gets printed.
func (p *ParseError) Error() string {
	var line, ident string
	if p.Line != 0 {
		line = fmt.Sprintf("%d:", p.Line)
	}
	if p.Ident != "" {
		ident = fmt.Sprintf(" %s:", p.Ident)
	}
	return fmt.Sprintf("%s:%s%s %s\n", p.File, line, ident, p.Err)
}

// newError creates ParseError from s
func (p *parser) newError(e error) *ParseError {
	return &ParseError{p.file, p.line, p.ident, p.value, e}
}

// Regexps for tokens
var (
	identRE  = regexp.MustCompile(`^[-_a-zA-Z][-_a-zA-Z0-9]*`)
	plainRE  = regexp.MustCompile(`^[^\pZ\pC"#'=\\]+`)
	quotedRE = regexp.MustCompile(`^"(?:[^\pC"\\]|\\[^\pC])*"`)
)

func eatSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func (p *parser) setValue(value string) error {
	for i := range p.vars {
		v := &p.vars[i]
		if p.ident == v.Name {
			if v.set {
				return p.newError(errAlreadyDef)
			}
			if !v.flagSet {
				if err := v.Val.Set(value); err != nil {
					return &ParseError{p.file, p.line,
						p.ident, p.value, err}
				}
			}
			v.set = true
			return nil
		}
	}
	return p.newError(errUnknownVar)
}

func (p *parser) parseLine(line string) error {
	line = eatSpace(line)
	if line == "" || line[0] == '#' {
		return nil
	}
	p.ident = identRE.FindString(line)
	line = eatSpace(line[len(p.ident):])
	if p.ident == "" || line == "" || line[0] != '=' {
		return p.newError(errSyntax)
	}
	line = eatSpace(line[1:])
	p.value = plainRE.FindString(line)
	unquoted := p.value
	if p.value == "" {
		p.value = quotedRE.FindString(line)
		var err error
		unquoted, err = strconv.Unquote(p.value)
		if err != nil {
			return p.newError(errSyntax)
		}
	}
	line = eatSpace(line[len(p.value):])
	if len(line) != 0 && line[0] != '#' {
		return p.newError(errSyntax)
	}
	return p.setValue(unquoted)
}

// Parse parses the configuration file from r according the description
// in vars and sets the variables pointed to to the values in the file.
// The filename is used in error messages; if empty, it's set to "stdin".
// It returns nil on success, ParseError on parsing error and something
// from the depths of io on actual real error.
//
// Parsing stops on the first error encountered.  Setting an unknown
// variable, setting a variable more than once or omitting a Var whose
// Required == true are errors.
//
// When parsing, the value gets unquoted if needed and the Var
// corresponding to the identifier is found.  Then the Set() method
// is called to set the Var.  If you need syntax validation, you
// should create your own Value type and return an error from Set()
// on invalid input.
//
// The parsing sequence implies that even when a number is desired,
// the quoted string "\x32\u0033" is the same as unquoted 23.
func Parse(r io.Reader, filename string, vars []Var) error {
	p := &parser{file: filename, vars: vars}
	if p.file == "" {
		p.file = "stdin"
	}
	if t, ok := r.(*bufio.Reader); ok {
		p.r = t
	} else {
		p.r = bufio.NewReader(r)
	}
	for {
		p.line++
		p.ident, p.value = "", ""
		buf, ispref, err := p.r.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		} else if ispref {
			return p.newError(errLineTooLong)
		}
		if err = p.parseLine(string(buf)); err != nil {
			return err
		}
	}
	for _, v := range p.vars {
		if v.Required && !v.set {
			return &ParseError{p.file, 0, v.Name, "", errReqNotSet}
		}
	}
	return nil
}
