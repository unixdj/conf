// Copyright 2001 Vadim Vygonets.  No rights reserved.
// Use of this source code is governed by WTFPL v2
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"github.com/unixdj/conf"
	"os"
	"regexp"
)

var (
	confFile = "example.conf"
	bval     bool
	sval     = "default value"
	nval     uint64
	netKey   []byte
	netKeyRE = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

type netKeyValue []byte

func (key *netKeyValue) Set(s string) error {
	if !netKeyRE.MatchString(s) {
		return errors.New("invalid key (must be 64 hexadecimal digits)")
	}
	fmt.Sscanf(s, "%x", key) // will succeed
	return nil
}

//func (key *netKeyValue) String() string { return fmt.Sprintf("%x", *key) }

var vars = []conf.Var{
	// cmd-line only:
	//{Flag: 'h', Val: (*conf.StringValue)(&confFile)},
	{Flag: 'c', Val: (*conf.StringValue)(&confFile)},
	// cmd-line and conf-file:
	{Flag: 's', Name: "string", Val: (*conf.StringValue)(&sval)},
	{Flag: 'n', Name: "number", Val: (*conf.Uint64Value)(&nval)},
	{Flag: 'b', Name: "bool", Val: (*conf.BoolValue)(&bval), Kind: conf.NoArg},
	{Flag: 'k', Name: "key", Val: (*netKeyValue)(&netKey), Required: true},
	// conf-file only:
}

func readConf(conffile string, vars []conf.Var) error {
	f, err := os.Open(conffile)
	if err != nil {
		return err
	}
	defer f.Close()
	return conf.Parse(f, conffile, vars)
}

func main() {
	fmt.Printf("*** start:\nconffile: %s\nstring: %s\nnumber: %d\nbool: %v\nkey: %x\n",
		confFile, sval, nval, bval, netKey)
	if err := conf.GetOptLongOnly(vars); err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	fmt.Printf("*** after GetOpt:\nconffile: %s\nstring: %s\nnumber: %d\nbool: %v\nkey: %x\n",
		confFile, sval, nval, bval, netKey)
	if err := readConf(confFile, vars[1:]); err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	fmt.Printf("*** after Parse:\nconffile: %s\nstring: %s\nnumber: %d\nbool: %v\nkey: %x\n",
		confFile, sval, nval, bval, netKey)
}
