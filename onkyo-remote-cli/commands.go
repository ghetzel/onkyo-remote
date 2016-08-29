package main

import (
	"fmt"
	"github.com/ghetzel/onkyo-remote"
	"regexp"
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

func MessageToCommand(subcommand string, m eiscp.Message) (*CommandInfo, *Value, error) {
	if cmd, ok := codeToCmd[m.Code()]; ok && cmd != nil {
		for i := range cmd.Values {
			if rx, err := regexp.Compile(`^` + cmd.Values[i].Code + `$`); err == nil {
				if rx.MatchString(m.Value()) {
					log.Debugf("%q: Value %+v matched", m.Value(), cmd.Values[i])
					value := &cmd.Values[i]
					value.Data = m.Value()

					// if matches := rx.FindStringSubmatch(m.Value()); len(matches) > 0 {
					// 	value.Data = matches[0]
					// } else {
					//  value.Data = m.Value()
					// }

					// switch value.Type {
					// case Hexadecimal:
					// 	if v, err := strconv.ParseInt(value.Data, 10, 32); err == nil {
					// 		value.Data = fmt.Sprintf("%02X", v)
					// 	} else {
					// 		return nil, nil, err
					// 	}
					// }

					log.Debugf("CALL: %s (%s): %s (%s) %q", cmd.Name, cmd.Code, value.Name, value.Code, value.Data)

					return cmd, value, nil
				}
			} else {
				return nil, nil, err
			}
		}

		return cmd, nil, nil
	} else {
		return nil, nil, fmt.Errorf("Command %q not found", m.Code())
	}
}
