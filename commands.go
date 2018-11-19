package main

//INCOMPLETE

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

type cmd_rights_check func(u user)
type cmd_func func(params map[string]param, args ...string)
type command struct {
	name          string
	desc          string
	params        map[string]param
	_r_check_func cmd_rights_check
	_func         cmd_func
}

func (c *command) run(u user, input string) {
	//TODO
}

func new_command(name, desc string, _r_check_func cmd_rights_check, _func cmd_func, params ...param) command {
	ret := command{name, desc, make(map[string]param), _r_check_func, _func}
	for _, p := range params {
		ret.params[p.name] = p
	}
	return ret
}
