package main

import (
	"fmt"
	"github.com/ghetzel/onkyo-remote"
	"strconv"
)

type ValueType int

const (
	Raw ValueType = iota
	Hexadecimal
)

type Value struct {
	Data        string
	Code        string
	Name        string
	Description string
	Type        ValueType
}

func (self *Value) String() string {
	switch self.Type {
	case Hexadecimal:
		if v, err := strconv.ParseInt(self.Data, 16, 32); err == nil {
			return fmt.Sprintf("%d", v)
		} else {
			return ``
		}
	default:
		return self.Data
	}
}

type CommandInfo struct {
	Zone        string
	Code        string
	Name        string
	Description string
	Values      []Value
}

var codeToCmd = map[string]*CommandInfo{}

func initCommands() {
	for i := range AllKnownCommands {
		cmd := &AllKnownCommands[i]
		codeToCmd[cmd.Code] = cmd
	}
}

func MessageToCommand(subcommand string, m eiscp.Message) (*CommandInfo, *Value) {
	if cmd, ok := codeToCmd[m.Code()]; ok && cmd != nil {
		for i := range cmd.Values {
			if cmd.Values[i].Code == subcommand {
				value := &cmd.Values[i]
				value.Data = m.Value()

				return cmd, value
			}
		}

		return cmd, nil
	} else {
		return nil, nil
	}
}
