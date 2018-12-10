package main

//INCOMPLETE

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"sort"
	"strconv"
	"strings"
)

type userdata struct {
	key, server string
	message     *discordgo.MessageCreate
	session     *discordgo.Session
}

type param_map map[string]interface{}

type param struct {
	name string
	desc string
	def  interface{}
}

func new_param(name, desc string, def interface{}) *param {
	return &param{name, desc, def}
}

func new_flag(name, desc string) param {
	return param{name, desc, false}
}

type cmd_rights_check func(u userdata) (bool, string)
type cmd_func func(params param_map, args string) string
type command struct {
	name          string
	desc          string
	usage         string
	params        map[string]*param
	_r_check_func cmd_rights_check
	_func         cmd_func
}

func (c *command) run(u userdata, args ...string) (err string) {
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
	params["userdata"] = u
	cmdarg := ""
	for i := 0; i < len(args); i++ {
		if args[i][0] == '-' {
			//flag or option
			if args[i][1] == '-' {
				//option
				param, ok := c.params[args[i][2:]]
				if !ok {
					return fmt.Sprintf("no such param '%v'", args[i][2:])
				}
				switch param.def.(type) {
				case bool:
					params[param.name] = true
				default:
					receivers = append(receivers, param.name)
				}
			} else {
				for i := 1; i < len(args[i]); i++ {
					//flag
					param, ok := c.params[args[i][i:i+1]]
					if !ok {
						return fmt.Sprintf("no such flag '%v'", args[i][2:])
					}
					switch param.def.(type) {
					case bool:
						params[param.name] = true
					default:
						receivers = append(receivers, param.name)
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
					params[curr.name] = args[i]
				case int:
					params[curr.name], err = strconv.Atoi(args[i])
					noerror(err)
				}
			} else {
				cmdarg += args[i] + " "
			}
		}
	}
	l := len(cmdarg)
	if l > 0 {
		cmdarg = cmdarg[:l-1]
	}
	c._func(params, cmdarg)
	return err
}

func new_command(name, desc, usage string, _r_check_func cmd_rights_check, _func cmd_func, params ...*param) *command {
	ret := command{name, desc, usage, make(map[string]*param), _r_check_func, _func}
	for _, p := range params {
		ret.params[p.name] = p
	}
	return &ret
}

type shell_repo struct {
	commands        map[string]*command
	params          map[string]*param
	checkfuncs      map[string]cmd_rights_check
	checkfuncs_desc map[string]string
	__initialised   bool
}

var shr shell_repo

func shell_repo_init() {
	shr.commands = make(map[string]*command)
	shr.params = make(map[string]*param)
	shr.checkfuncs = make(map[string]cmd_rights_check)
	shr.checkfuncs_desc = make(map[string]string)
	add_command := func(name, desc, usage string, _r_check_func cmd_rights_check, _func cmd_func, params ...*param) {
		shr.commands[name] = new_command(name, desc, usage, _r_check_func, _func, params...)
	}
	add_uparam := func(name, desc string, def interface{}) {
		shr.params[name] = new_param(name, desc, def)
	}
	add_ucfunc := func(name, desc string, f cmd_rights_check) {
		shr.checkfuncs[name] = f
		shr.checkfuncs_desc[name] = desc
	}
	uparam := func(name string) *param {
		ret, ok := shr.params[name]
		assert(ok, "param not found: "+name)
		return ret
	}
	ucfunc := func(name string) cmd_rights_check {
		ret, ok := shr.checkfuncs[name]
		assert(ok, "checkfunc not found: "+name)
		return ret
	}
	//----------------------------//
	//----------ucfuncs-----------//
	//----------------------------//
	add_ucfunc("p_root", "requires root permissions", func(u userdata) (bool, string) {
		if !Permissions_check(u.message.Author, PERMISSIONS_SUPERUSER, u.server) {
			return false, "permission denied"
		}
		return true, ""
	})
	add_ucfunc("p_admin", "requires admin permissions", func(u userdata) (bool, string) {
		if !Permissions_check(u.message.Author, PERMISSIONS_ADMIN, u.server) {
			return false, "permission denied"
		}
		return true, ""
	})
	add_ucfunc("p_player", "requires player permissions", func(u userdata) (bool, string) {
		if u.key == "" {
			return false, "permission denied"
		}
		return true, ""
	})
	add_ucfunc("p_any", "requires no permissions", func(u userdata) (bool, string) { return true, "" })
	//----------------------------//
	//----------uparams-----------//
	//----------------------------//
	add_uparam("preply", "if set, bot will reply in private channel (opposed to replying where command was entered)", false)
	add_uparam("server", "overrides chosen server", "NOOVERRIDE")
	add_uparam("verbose", "in some cases makes commands generate less output", false)
	//----------------------------//
	//----------commands----------//
	//----------------------------//
	add_command("list",
		"prints specified LIST (candidates: admins, bans, users, roles, moderators, sdonators, commands)",
		"LIST", ucfunc("p_any"), func(params param_map, args string) (ret string) {
			switch args {
			case "bans", "users":
				ok, ret := ucfunc("p_admin")(params["userdata"].(userdata))
				if !ok {
					return ret
				}
			}
			_userdata := params["userdata"].(userdata)
			server := _userdata.server
			if params["server"].(string) != "NOOVERRIDE" {
				server = params["server"].(string)
			}
			if server == "" {
				server = "ALL"
			}
			_ckey := params["ckey"].(string)
			switch args {
			case "admins":
				ret = "known admins:\n"
				if server == "ALL" {
					for s, a := range Known_admins {
						ret += "[" + s + "]: " + strings.Join(a, ", ") + "\n"
					}
				} else {
					a, ok := Known_admins[server]
					if !ok {
						return "no entry for this server: " + server
					}
					ret += strings.Join(a, ", ")
				}
				return ret
			case "bans":
				defer logging_recover("list bans")
				defer onerror(func() {
					ret = "ERROR"
				})
				{
					ret = "\n"
					var ckey, admin, reason string
					var bt int
					if _ckey != "" {
						ckey = ckey_simplifier(_ckey)
						closure_callback := func() {
							bantype := make([]string, 0)
							if bt&BANTYPE_OOC != 0 {
								bantype = append(bantype, BANSTRING_OOC)
							}
							if bt&BANTYPE_COMMANDS != 0 {
								bantype = append(bantype, BANSTRING_COMMANDS)
							}
							bantypestring := strings.Join(bantype, ", ")
							ret += fmt.Sprintf("%v banned from %v by %v with reason \"%v\"\n", ckey, bantypestring, admin, reason)
						}
						db_template("select_bans_ckey").query(ckey).parse(closure_callback, &bt, &admin, &reason)
						if ret == "\n" {
							ret = "no bans currently active"
						}
						return ret
					}
					closure_callback := func() {
						bantype := make([]string, 0)
						if bt&BANTYPE_OOC != 0 {
							bantype = append(bantype, BANSTRING_OOC)
						}
						if bt&BANTYPE_COMMANDS != 0 {
							bantype = append(bantype, BANSTRING_COMMANDS)
						}
						bantypestring := strings.Join(bantype, ", ")
						ret += fmt.Sprintf("%v banned from %v by %v with reason \"%v\"\n", ckey, bantypestring, admin, reason)
					}
					db_template("select_bans").query().parse(closure_callback, &ckey, &bt, &admin, &reason)
					if ret == "\n" {
						ret = "no bans currently active"
					}
					return ret
				}
			case "users":
				if _ckey == "" {
					rep := "registered users:\n"
					for login, ckey := range local_users {
						rep += fmt.Sprintf("<@!%s> -> %s\n", login, ckey)
					}
					Discord_private_message_send(_userdata.message.Author, rep)
					return "sent to PM"
				} else {
					for login, ckey := range local_users {
						if ckey == _ckey {
							user, err := _userdata.session.User(login)
							if err != nil {
								return fmt.Sprintf("Failed to find user, showing ID instead: %s -> %s", login, ckey)
							}
							return fmt.Sprintf("%s -> %s", user.String(), ckey)
						}
					}
					return "not found"
				}
			case "roles":
				guild := Get_guild(_userdata.session, _userdata.message)
				if guild == "" {
					return "failed to retrieve guild"
				}
				groles, err := _userdata.session.GuildRoles(guild)
				if err != nil {
					return "failed to retrieve rolelist"
				}
				plr, ok := discord_player_roles[guild]
				if !ok {
					plr = "NONE"
				} else {
					for _, k := range groles {
						if k.ID == plr {
							plr = k.Name
							break
						}
					}
				}
				adm := ""
				adms, ok := discord_admin_roles[guild]
				if !ok {
					adm = "\nNONE"
				} else {
					for srv, ar := range adms {
						for _, k := range groles {
							if k.ID == ar {
								adm += "\n" + srv + " admin -> " + k.Name
								break
							}
						}
					}
				}
				sub := ""
				subs, ok := discord_subscriber_roles[guild]
				if !ok {
					sub = "\nNONE"
				} else {
					for srv, sr := range subs {
						for _, k := range groles {
							if k.ID == sr {
								sub += "\n" + srv + " subscriber -> " + k.Name
								break
							}
						}
					}
				}
				return "\nplayer -> " + plr + adm + sub
			case "moderators":
				defer onerror(func() { ret = "db request failed" })
				var ckey string
				db_template("select_moderators").query().parse(func() { ret += " `" + ckey + "`" }, &ckey)
				return "Current moderators: " + ret
			case "sdonators":
				if _ckey == "" {
					return "\n" + list_donators(server)
				} else {
					return "\n" + get_donator(server, _ckey)
				}
			case "commands":
				ret = ""
				verbose := params["verbose"].(bool)
				cmdsl := make([]string, 0)
				for c := range shr.commands {
					cmdsl = append(cmdsl, c)
				}
				sort.Strings(cmdsl)
				for _, c := range cmdsl {
					ret += "\n" + c + ""
					if !verbose {
						ret += ": " + shr.commands[c].desc
					}
				}
			case "":
				ret = "no list specified"
			default:
				ret = "no such list"
			}
			return ret
		}, uparam("server"), new_param("ckey", "filter bans, users or sdonators list by supplied ckey", ""), uparam("verbose"))
	shr.__initialised = true
}

//str without prefixing '$'
func shell_handler(u userdata, str string) string {
	//TODO: shell?
	strspl := strings.Fields(str)
	cmd, ok := shr.commands[strspl[0]]
	if !ok {
		return "no such command"
	}
	return cmd.run(u, strspl[1:]...)
}
