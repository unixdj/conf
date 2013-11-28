// Copyright 2012 Vadim Vygonets
// This program is free software. It comes without any warranty, to
// the extent permitted by applicable law. You can redistribute it
// and/or modify it under the terms of the Do What The Fuck You Want
// To Public License, Version 2, as published by Sam Hocevar. See
// the LICENSE file or http://sam.zoy.org/wtfpl/ for more details.

/*
Package conf parses simple configuration files and command line
arguments.

Command line processing is described under GetOpt, GetOptLong
and GetOptLongOnly.

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
