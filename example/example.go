package main

import (
	"errors"
	"fmt"
	"github.com/unixdj/conf"
	"os"
	"regexp"
)

var (
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

func readConf(conffile string) error {
	f, err := os.Open(conffile)
	if err != nil {
		return err
	}
	defer f.Close()
	return conf.Parse(f, conffile, []conf.Var{
		{Name: "string", Val: (*conf.StringValue)(&sval)},
		{Name: "number", Val: (*conf.Uint64Value)(&nval)},
		{Name: "key", Val: (*netKeyValue)(&netKey), Required: true},
	})
}

func main() {
	if err := readConf("example.conf"); err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	fmt.Printf("string: %s\nnumber: %d\nkey: %x\n", sval, nval, netKey)
}
