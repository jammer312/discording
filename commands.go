package main

//INCOMPLETE

import (
	"fmt"
	"strconv"
	"strings"
)

type user struct {
	key, discord_id string
}

type param_map map[string]interface{}

type param struct {
	name string
	desc string
	def  interface{}
}

func new_param(name, desc string, def interface{}) param {
	return param{name, desc, def}
}

func new_flag(name, desc string) param {
	return param{name, desc, false}
}

type cmd_rights_check func(u user) (bool, string)
type cmd_func func(params map[string]param, args string)
type command struct {
	name          string
	desc          string
	params        map[string]param
	_r_check_func cmd_rights_check
	_func         cmd_func
}

func (c *command) run(u user, input string) (err string) {
	defer recover()
	if ok, reason := c._r_check_func(u); !ok {
		return reason
	}
	receivers := make([]string, 0)
	lastr := 0
	params := make(param_map)
	for pn, p := range c.params {
		params[pn] = p.def
	}
	chunks := strings.Fields(input)
	cmdarg := ""
	for i := 0; i < len(chunks); i++ {
		if chunks[i][0] == '-' {
			//flag or option
			if chunks[i][1] == '-' {
				//option
				param, ok := c.params[chunks[i][2:]]
				if !ok {
					return fmt.Sprintf("no such param '%v'", chunks[i][2:])
				}
				switch param.def.(type) {
				case bool:
					params[param.name] = true
				default:
					receivers = append(receivers, param)
				}
			} else {
				for i := 1; i < len(chunks[i]); i++ {
					//flag
					param, ok := c.params[chunks[i][i:i+1]]
					if !ok {
						return fmt.Sprintf("no such flag '%v'", chunks[i][2:])
					}
					switch param.def.(type) {
					case bool:
						params[param.name] = true
					default:
						receivers = append(receivers, param)
					}
				}
			}
		} else {
			if len(receivers) > lastr {
				curr := c.params[receivers[lastr]]
				lastr++
				var err error
				switch curr.def.(type) {
				case string:
					params[curr.name] = chunks[i]
				case int:
					params[curr.name], err = strconv.Atoi(chunks[i])
					noerror(err)
				}
			} else {
				cmdarg += chunks[i]
			}
		}
	}
}

func new_command(name, desc string, _r_check_func cmd_rights_check, _func cmd_func, params ...param) command {
	ret := command{name, desc, make(map[string]param), _r_check_func, _func}
	for _, p := range params {
		ret.params[p.name] = p
	}
	return ret
}
