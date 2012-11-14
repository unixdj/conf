// Copyright 2001 Vadim Vygonets.  No rights reserved.
// Use of this source code is governed by WTFPL v2
// that can be found in the LICENSE file.

/*
Package conf parses simple configuration files.

Configuration file syntax (see Parse() for semantics):

The file is composed of lines of UTF-8 text, each no longer than 4KB.
Comments start with '#' and continue to end of line.
Whitespace (Unicode character class Z) between tokens is ignored.
Configuration settings look like this:

	ident = value

Identifiers start with an ASCII letter, dash ('-') or underscore ('_'),
and continue with zero or more ASCII letters, ASCII digits, dashes or
underscores.  That is, they match /[-_a-zA-Z][-_a-zA-Z0-9]/.

Values may be plain or quoted.  Plain values may have any character in
them besides space (Unicode character class Z), control characters
(Unicode character class C), or any of '"', '#', `'`, '=', `\`.

Quoted values are enclosed in double quotes (like "this") and obey Go
quoted string rules.  They may not include Unicode control characters.
Any character except '"' and `\` stands for itself.  Backslash escapes
\a, \b, \f, \n, \r, \t, \v, \", \\, \337, \xDF, \u1A2F and \U00104567 are
accepted.  Quoted values, unlike plain ones, can be empty ("").

The rule about control characters means that tabs inside quoted strings
must be replaced with "\t" (or "\U00000009" or whatever).

Example:

	ipv6-addr = [::1]:23         # Look ma, no quotes!
	file      = /etc/passwd      # Comments after settings are OK.
	--        = "hello, world\n" # Variables can have strange names.

ABNF:

	; The language's charset is Unicode, encoding is UTF-8.

	file         = *line
	line         = [assignment] [comment] nl
	assignment   = ows ident equals value
	value        = plain-value / quoted-value

	; The token <opt-space> can appear anywhere and is ignored.

	; Tokens:

	comment      = ows "#" *ctext
	ident        = ident-alpha *ident-alnum
	equals       = ows "=" ows
	plain-value  = 1*ptext
	quoted-value = DQUOTE *(qtext / quoted-pair) DQUOTE
	ows          = *WSP
	nl           = [CR] LF

	ident-alnum  = ident-alpha / DIGIT
	ident-alpha  = ascii-alpha / "-" / "_"

	quoted-pair  = BACKSLASH quoted-char
	quoted-char  = escaped-char / byte-val / unicode-val
	escaped-char = %x61 / %x62 / %x66 / %x6E / %x72 / %x74 / %x76
		     / DQUOTE / BACKSLASH	; [abfnrtv"\\]
	byte-val     = 3octal-digit		; [0-7]{3}
		     / %x78 2HEXDIG		; x[0-9A-Fa-f]{2}
	unicode-val  = %x75 4HEXDIG		; u[0-9A-Fa-f]{4}
		     / %x55 8HEXDIG		; U[0-9A-Fa-f]{8}

	ctext        = %x00-09 / %x0B-10FFFF	; any CHAR excluding LF
	ptext        = <any CHAR excluding WSP, CTL,
			DQUOTE, "#", "'", "=", BACKSLASH>
	qtext        = <any CHAR excluding CTL, DQUOTE, BACKSLASH>
	ascii-alpha  = %x41-5A / %x61-7A	; [A-Za-z]
	octal-digit  = %x30-37			; [0-7]
	HEXDIG       = DIGIT / %x41-56 / %x61-66; [0-9A-Fa-f]
	DIGIT        = %x30-39			; [0-9]
	WSP          = <any CHAR from Unicode character class Z excluding LF>
	CTL          = %x00-1F / %x7F-9F	; Unicode character class C
	DQUOTE       = %x22			; "
	BACKSLASH    = "\"			; \
	CHAR         = %x00-10FFFF		; any Unicode character
*/
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
// Syntax: 0/false/off/no/disabled 1/true/on/yes/enabled (case insensitive).
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
	case strInList(s, []string{"0", "false", "off", "no", "disabled"}):
		*v = false;
	case strInList(s, []string{"1", "true", "on", "yes", "enabled"}):
		*v = true;
	default:
		return errors.New(syntaxError)
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

// Var describes a configuration variable and has pointers to corresponding
// (Go) variables.  Slice of Var is used for calling Parse().
type Var struct {
	Name     string // name of configuration variable
	Val      Value  // Value to set
	Required bool   // variable is required to be set in conf file
	set      bool   // has been set
}

type parser struct {
	r     *bufio.Reader
	file  string
	line  int
	ident string
	value string
	vars  []Var
}

const (
	syntaxError = "syntax error"
)

// ParseError represents the error.
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
func (p *parser) newError(s string) *ParseError {
	return &ParseError{p.file, p.line, p.ident, p.value, errors.New(s)}
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
				return p.newError("already defined")
			}
			if err := v.Val.Set(value); err != nil {
				return &ParseError{p.file, p.line, p.ident,
					p.value, err}
			}
			v.set = true
			return nil
		}
	}
	return p.newError("unknown variable")
}

func (p *parser) parseLine(line string) error {
	line = eatSpace(line)
	if line == "" || line[0] == '#' {
		return nil
	}
	p.ident = identRE.FindString(line)
	line = eatSpace(line[len(p.ident):])
	if p.ident == "" || line == "" || line[0] != '=' {
		return p.newError(syntaxError)
	}
	line = eatSpace(line[1:])
	p.value = plainRE.FindString(line)
	unquoted := p.value
	if p.value == "" {
		p.value = quotedRE.FindString(line)
		var err error
		unquoted, err = strconv.Unquote(p.value)
		if err != nil {
			return p.newError(syntaxError)
		}
	}
	line = eatSpace(line[len(p.value):])
	if len(line) != 0 && line[0] != '#' {
		return p.newError(syntaxError)
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
			return p.newError("line too long")
		}
		if err = p.parseLine(string(buf)); err != nil {
			return err
		}
	}
	for _, v := range p.vars {
		if v.Required && !v.set {
			return &ParseError{p.file, 0, v.Name, "",
				errors.New("required but not set")}
		}
	}
	return nil
}
