package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ghetzel/onkyo-remote"
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

func (self *CommandInfo) String() string {
	values := make([]string, 0)

	if len(self.Values) > 0 {
		for _, value := range self.Values {
			v := value.String()

			if len(v) > 0 {
				values = append(values, fmt.Sprintf("%s:%s", v, value.Name))
			}
		}
	}

	return fmt.Sprintf("%s\t%s\t%s\t%d\t%s",
		self.Code,
		self.Name,
		self.Zone,
		len(values),
		strings.Join(values, "\t"))
}

func MessageToCommand(subcommand string, m onkyo.Message) (*CommandInfo, *Value, error) {
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
