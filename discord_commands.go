package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"math/rand" //dice
	"sort"
	"strconv"
	"strings"
	"time" //dice
)

type dcfunc func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string

type Dcommand struct {
	Command         string
	Minargs         int
	Permlevel       int
	Usage           string
	Desc            string
	functional      dcfunc
	Temporary       int
	Server_specific bool
	Categories      []string
}

var known_commands map[string]*Dcommand
var known_categories map[string][]*Dcommand

func category_register_command(cat string, cmd *Dcommand) {
	ct, ok := known_categories[cat]
	if !ok {
		known_categories[cat] = []*Dcommand{cmd}
	} else {
		known_categories[cat] = append(ct, cmd)
	}
}

func category_printout(cat string, perms int) string {
	ret := "**" + cat + "**:\n"
	ct, ok := known_categories[cat]
	if !ok {
		return ret + "no such category"
	}
	cmds := make([]string, 0)
	for _, dc := range ct {
		if dc.Permlevel > perms {
			continue
		}
		cmdstr := "	`" + dc.Command + "`"
		if dc.Server_specific {
			cmdstr += " *SS*"
		}
		cmds = append(cmds, cmdstr)
	}
	cmdsstr := strings.Join(cmds, "\n")
	if cmdsstr == "" {
		cmdsstr = "no available commands"
	}
	return ret + cmdsstr
}

func Register_command(in *Dcommand) {
	known_commands[in.Command] = in
	if in.Categories == nil {
		category_register_command("unsorted", in)
	}
	for _, v := range in.Categories {
		category_register_command(v, in)
	}
}

func (d *Dcommand) Exec(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
	return d.functional(session, message, args, server)
}
func (d *Dcommand) Usagestr() string {
	return Discord_command_character + d.Command + " " + d.Usage
}

func init() {
	known_commands = make(map[string]*Dcommand)
	known_categories = make(map[string][]*Dcommand)
	// ------------
	Register_command(&Dcommand{
		Command:   "list_admins",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "list known admin ckeys",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := "known admins:\n"
			if server == "" {
				for s, a := range Known_admins {
					ret += "**" + s + "**: " + strings.Join(a, ", ") + "\n"
				}
			} else {
				a, ok := Known_admins[server]
				if !ok {
					return "no entry for this server: " + server
				}
				ret += strings.Join(a, ", ")
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "check_permissions",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!id]",
		Desc:      "check permissions for given id",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			usr, err := dsession.User(args[0])
			if err != nil {
				return "fail"
			}
			switch get_permission_level(usr, server) {
			case PERMISSIONS_NONE:
				return "none"
			case PERMISSIONS_REGISTERED:
				return "registered"
			case PERMISSIONS_ADMIN:
				return "admin"
			case PERMISSIONS_SUPERUSER:
				return "superuser"
			default:
				return "unknown"
			}
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "reload_admins",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "",
		Desc:      "sync admins list with hub",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			if server == "" {
				Load_admins()
			} else {
				Load_admins_for_server(server)
			}
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "login",
		Minargs:   0,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "",
		Desc:      "receive channel permissions according to your ckey rank",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			channel, err := session.Channel(message.ChannelID)
			if err != nil {
				log.Println("Shiet: ", err)
				return "failed to retrieve channel"
			}
			if login_user(channel.GuildID, message.Author.ID) {
				return "successfully logged in as " + local_users[message.Author.ID]
			}
			return "login failed"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "logoff",
		Minargs:   0,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "",
		Desc:      "remove channel permissions according to your ckey rank",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			channel, err := session.Channel(message.ChannelID)
			if err != nil {
				log.Println("Shiet: ", err)
				return "failed to retrieve channel"
			}
			if logoff_user(channel.GuildID, message.Author.ID) {
				return "successfully logged off"
			}
			return "logoff failed"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "whoami",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "printout ckey you account is linked to",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ckey := local_users[message.Author.ID]
			if ckey == "" {
				return "you're not registered"
			}
			return "you're registered as " + ckey
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "register",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "(re)bind your discord account to byond ckey",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			Remove_token("register", message.Author.ID)
			id := Create_token("register", message.Author.ID)
			if id == "" {
				return "failed for some reason, ask maintainer to investigate"
			}
			Discord_private_message_send(message.Author, "Use `Bot token` in `OOC` tab on game server with following token: `"+id+"` to complete registration. Afterwards you can use `!login` to gain ooc permissions in discord guild.")
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "list_registered",
		Minargs:   0,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "",
		Desc:      "list registered users in format [discord nick] -> [ckey]",
		Temporary: DEL_DEFAULT,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			rep := "registered users:\n"
			for login, ckey := range local_users {
				rep += fmt.Sprintf("<@!%s> -> %s\n", login, ckey)
			}
			Discord_private_message_send(message.Author, rep)
			return "sent to PM"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "who",
		Minargs:         0,
		Permlevel:       PERMISSIONS_NONE,
		Usage:           "",
		Desc:            "list players currently on server",
		Server_specific: true,
		Temporary:       DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			br := Byond_query(server, "who", false)
			preret := strings.Split(br.String(), "\n")
			if len(preret) <= 2 {
				return strings.Join(preret, "\n")
			}
			sort.Strings(preret[1 : len(preret)-1])
			return strings.Join(preret, "\n")

		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "channel_here",
		Minargs:   2,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!server] [!channel type]",
		Desc:      "create and/or bind provided channel type to discord channel",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			server = args[0]
			if Update_known_channel(server, args[1], message.ChannelID, guild) {
				return "changed `" + Dweaksanitize(server) + "@" + Dweaksanitize(args[1]) + "` channel to <#" + message.ChannelID + ">"
			} else {
				return "failed to change `" + Dweaksanitize(server) + "@" + Dweaksanitize(args[1]) + "` channel to <#" + message.ChannelID + ">"
			}
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "channel_list",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "",
		Desc:      "list known channel types and channels they're bound to",
		Temporary: DEL_NEVER,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			return List_known_channels()
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "channel_remove",
		Minargs:   0,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[?server] [?channel_type]",
		Desc:      "unbind either provided channel type or else one bound to receiving discord channel and forget about it",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if len(args) < 2 {
				tch, ok := known_channels_id_t[message.ChannelID]
				if !ok {
					return "no channel bound here"
				}
				args = append(args[:0], tch.server, tch.generic_type)
			}
			if Remove_known_channels(args[0], args[1], guild) {
				return "removed `" + Dweaksanitize(args[0]) + "@" + Dweaksanitize(args[1]) + "`"
			} else {
				return "failed to remove `" + Dweaksanitize(args[0]) + "@" + Dweaksanitize(args[1]) + "`"
			}

		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ah",
		Minargs:         2,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "[!ckey] [!message]",
		Desc:            "sends adminPM containing [!message] to [!ckey]",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			Byond_query(server, "adminhelp&admin="+Bquery_convert(local_users[message.Author.ID])+"&ckey="+Bquery_convert(args[0])+"&response="+Bquery_convert(strings.Join(args[1:], " ")), true)
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahr",
		Minargs:         1,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "[!message]",
		Desc:            "replies to last AHELP with [!message]",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			if last_ahelp[server] == "" {
				return ""
			}
			Byond_query(server, "adminhelp&admin="+Bquery_convert(local_users[message.Author.ID])+"&ckey="+Bquery_convert(last_ahelp[server])+"&response="+Bquery_convert(strings.Join(args, " ")), true)
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahl",
		Minargs:         1,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "[!ckey]",
		Desc:            "locks your ahelp to [!ckey]",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			_, ok := discord_ahelp_locks[server]
			if !ok {
				discord_ahelp_locks[server] = make(map[string]string)
			}
			discord_ahelp_locks[server][message.Author.ID] = args[0]
			return "locked to " + args[0]
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahlr",
		Minargs:         0,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "",
		Desc:            "locks your ahelp to last AHELP",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			if last_ahelp[server] == "" {
				return "no recent AHELP found"
			}
			_, ok := discord_ahelp_locks[server]
			if !ok {
				discord_ahelp_locks[server] = make(map[string]string)
			}
			discord_ahelp_locks[server][message.Author.ID] = last_ahelp[server]
			return "locked to " + last_ahelp[server]
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahm",
		Minargs:         1,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "[!message]",
		Desc:            "sends admin [!message] to locked ckey (see 'ahl' and 'ahlr')",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			_, ok := discord_ahelp_locks[server]
			if !ok {
				return "no active lock"
			}
			lock := discord_ahelp_locks[server][message.Author.ID]
			if lock == "" {
				return "no active lock"
			}
			Byond_query(server, "adminhelp&admin="+Bquery_convert(local_users[message.Author.ID])+"&ckey="+lock+"&response="+Bquery_convert(strings.Join(args, " ")), true)
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahu",
		Minargs:         0,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "",
		Desc:            "unlocks your ahelp",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			_, ok := discord_ahelp_locks[server]
			if !ok {
				discord_ahelp_locks[server] = make(map[string]string)
			}
			discord_ahelp_locks[server][message.Author.ID] = ""
			return "ahelp unlocked"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "ahl?",
		Minargs:         0,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "",
		Desc:            "shows your current ahelp lock",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			_, ok := discord_ahelp_locks[server]
			if !ok {
				return "no active lock"
			}
			lock := discord_ahelp_locks[server][message.Author.ID]
			if lock == "" {
				return "no active lock"
			}
			return lock
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "toggle_ooc",
		Minargs:         0,
		Permlevel:       PERMISSIONS_ADMIN,
		Usage:           "",
		Desc:            "globally toggle ooc",
		Temporary:       DEL_NEVER,
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			Byond_query(server, "OOC", true)
			return "toggled global OOC"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "help",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "print list of commands available to you",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			if len(args) < 1 {
				cats := make([]string, 0)
				for k, _ := range known_categories {
					cats = append(cats, k)
				}
				return "Type `!help [category]` to print available commands fitting in provided category\nor `!help all` to print all available commands\nValid categories are: `" + strings.Join(cats, "` `") + "`\nSS means that command requires channel to be bound to game server"
			}
			perms := get_permission_level(message.Author, server)
			ret := ""
			if args[0] == "all" {
				strs := make([]string, 0)
				for cat, _ := range known_categories {
					strs = append(strs, category_printout(cat, perms))
				}
				ret = strings.Join(strs, "\n")
			} else {
				ret = category_printout(args[0], perms)
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "help_pm",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "print list of commands available to you in private message",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			call, creg, cadm, csup := make([]string, 0), make([]string, 0), make([]string, 0), make([]string, 0)
			ret := ""
			user := message.Author
			for comm, dcomm := range known_commands {
				switch dcomm.Permlevel {
				case PERMISSIONS_NONE:
					call = append(call, comm)
				case PERMISSIONS_REGISTERED:
					creg = append(creg, comm)
				case PERMISSIONS_ADMIN:
					cadm = append(cadm, comm)
				case PERMISSIONS_SUPERUSER:
					csup = append(csup, comm)
				}
			}
			//sort it in alphabetical, because otherwise order is random which is no good
			sort.Strings(call)
			sort.Strings(creg)
			sort.Strings(cadm)
			sort.Strings(csup)
			if Permissions_check(user, PERMISSIONS_NONE, "") {
				ret += "\n**Generic commands:**\n" + strings.Join(call, "\n")
			}
			if Permissions_check(user, PERMISSIONS_REGISTERED, "") {
				ret += "\n**Commands, available to registered users:**\n" + strings.Join(creg, "\n")
			}
			if Permissions_check(user, PERMISSIONS_ADMIN, "") {
				ret += "\n**Admin commands:**\n" + strings.Join(cadm, "\n")
			}
			if Permissions_check(user, PERMISSIONS_SUPERUSER, "") {
				ret += "\n**Superuser commands:**\n" + strings.Join(csup, "\n")
			}
			Discord_private_message_send(user, ret)
			return "sent to PM"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "usage",
		Minargs:   1,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "[!cmd_name]",
		Desc:      "print description for provided command",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			cmd_name := args[0]
			dcmd, ok := known_commands[cmd_name]
			if !ok {
				return "no such command"
			}
			if !Permissions_check(message.Author, dcmd.Permlevel, "") {
				return "missing required permissions"
			}
			return dcmd.Usagestr() + "\n" + dcmd.Desc
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "adminwho",
		Minargs:         0,
		Permlevel:       PERMISSIONS_NONE,
		Usage:           "",
		Desc:            "prints admins currently on server",
		Temporary:       DEL_LONG,
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			br := Byond_query(server, "adminwho", false)
			str := br.String()
			if str == "NULL" {
				str = "strange shit happened, unable to get adminwho result"
			}
			return str
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "role_update",
		Minargs:   2,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!type] [!role_slap] [?server]",
		Desc:      "adds/updates [!role_slap] role of [!type] type; correct roles are '" + ROLE_PLAYER + "' and '" + ROLE_ADMIN + "'",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			tp, slap := args[0], args[1]
			srv := ""
			if tp == "" || slap == "" {
				return "incorrect usage"
			}
			if tp != ROLE_PLAYER && len(args) < 3 {
				return "this role requires server"
			} else if tp != ROLE_PLAYER {
				srv = args[2]
			}
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if update_known_role(guild, tp, slap[3:len(slap)-1], srv) {
				return "OK"
			}
			return "FAIL"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "role_remove",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!type] [?server]",
		Desc:      "removes role of [!type] type",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			tp := args[0]
			srv := ""
			if tp == "" {
				return "incorrect usage"
			}
			if tp != ROLE_PLAYER && len(args) < 2 {
				return "this role requires server"
			} else {
				srv = args[1]
			}
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if remove_known_role(guild, tp, srv) {
				return "OK"
			}
			return "FAIL"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "role_list",
		Minargs:   0,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "",
		Desc:      "lists known roles for this guild",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			groles, err := session.GuildRoles(guild)
			if err != nil {
				log.Println("ERROR: ", err)
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
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "ckey",
		Minargs:   1,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "[!@mention]",
		Desc:      "returns ckey of mentioned user",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			args = strings.Fields(message.Content[1:])
			mention := args[1]
			if len(mention) < 4 {
				return "incorrect input"
			}
			userid := mention[2 : len(mention)-1]
			ckey := local_users[userid]
			if ckey == "" {
				userid = userid[1:]
				ckey = local_users[userid]
				if ckey == "" {
					return "no bound ckey"
				}
			}
			return ckey
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "baninfo",
		Minargs:   0,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "",
		Desc:      "prints your discord bans, if any",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := check_bans_readable(message.Author, server, ^0)
			if ret == "" {
				return "you have no active bans here"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "ban_apply",
		Minargs:   3,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "[!ckey] [!type] [!reason]",
		Desc:      "update existing ban's type or create new with following reason, valid types are " + BANSTRING_OOC + " and " + BANSTRING_COMMANDS,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ckey := args[0]
			bantypestr := args[1]
			bantype := 0
			switch bantypestr {
			case BANSTRING_OOC:
				bantype = BANTYPE_OOC
			case BANSTRING_COMMANDS:
				bantype = BANTYPE_COMMANDS
			default:
				return "incorrect type"
			}
			reason := strings.Join(args[2:], " ")
			succ, st := update_ban(ckey, reason, message.Author, bantype)
			if succ {
				return "OK " + st
			}
			return "FAIL " + st
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "ban_lift",
		Minargs:   2,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "[!ckey] [!type]",
		Desc:      "remove existing ban issued by you or lower-ranked person, if any; valid types are " + BANSTRING_OOC + " and " + BANSTRING_COMMANDS,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ckey := strings.ToLower(args[0])
			bantypestr := args[1]
			bantype := 0
			switch bantypestr {
			case BANSTRING_OOC:
				bantype = BANTYPE_OOC
			case BANSTRING_COMMANDS:
				bantype = BANTYPE_COMMANDS
			default:
				num, err := strconv.Atoi(bantypestr)
				if err != nil {
					return "incorrect type"
				}
				bantype = num
			}
			succ, st := remove_ban(ckey, bantype, message.Author)
			if succ {
				return "OK " + st
			}
			return "FAIL " + st
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "ban_list",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "[?ckey]",
		Desc:      "prints existing bans",
		Temporary: DEL_NEVER,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			defer logging_recover("b_l")
			ret := "\n"
			var ckey, admin, reason string
			var bt int
			if len(args) > 0 {
				ckey := strings.ToLower(args[0])
				closure_callback := func() {
					bantype := make([]string, 0)
					if bt&BANTYPE_OOC != 0 {
						bantype = append(bantype, BANSTRING_OOC)
					}
					if bt&BANTYPE_COMMANDS != 0 {
						bantype = append(bantype, BANSTRING_COMMANDS)
					}
					bantypestring := strings.Join(bantype, ", ")
					ret += fmt.Sprintf("%v banned from %v by %v with reason `%v`\n", ckey, bantypestring, admin, reason)
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
				ret += fmt.Sprintf("%v banned from %v by %v with reason `%v`\n", ckey, bantypestring, admin, reason)
			}
			db_template("select_bans").query().parse(closure_callback, &ckey, &bt, &admin, &reason)
			if ret == "\n" {
				ret = "no bans currently active"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "sub",
		Minargs:         0,
		Permlevel:       PERMISSIONS_REGISTERED,
		Usage:           "",
		Desc:            "assigns you 'subscriber' role that gets notification each time round is about to start",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := "FAIL"
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if subscribe_user(guild, message.Author.ID, server) {
				ret = "OK"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "sub_once",
		Minargs:         0,
		Permlevel:       PERMISSIONS_REGISTERED,
		Usage:           "",
		Desc:            "tells bot to notify you next time round is about to start",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := "FAIL"
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if subscribe_user_once(guild, message.Author.ID, server) {
				ret = "OK"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:         "unsub",
		Minargs:         0,
		Permlevel:       PERMISSIONS_REGISTERED,
		Usage:           "",
		Desc:            "removes your 'subscriber' role that gets slapped each time round is about to start",
		Server_specific: true,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := "FAIL"
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if unsubscribe_user(guild, message.Author.ID, server) {
				ret = "OK"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "info",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "prints some info about bot",
		Temporary: DEL_LONG,
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ret := "opensource golang bot for ss13<->discord\n"
			ret += "github repo: https://github.com/jammer312/discording\n"
			ret += "main discord guild: https://discord.gg/T3kZZNR\n"
			ret += "try typing `!register` , `!help` and `!usage`\n"
			ret += "or maybe check out https://forum.ss13.ru/index.php?showtopic=18451"
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "dice",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Temporary: DEL_NEVER,
		Usage:     "[?sides] [?times] [?mode]",
		Desc:      "Rolls dice with [0<sides<312] (default: 6) sides [0<times<312] (default: 1) times and outputs result based on given [mode] (default: SUM). Possible modes - SUM, MOD, AVG",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			inputlen := len(args)
			sides, times := 6, 1
			mode := "SUM"
			var err error
			if inputlen > 0 {
				sides, err = strconv.Atoi(args[0])
				if err != nil {
					return "failed to parse input: " + fmt.Sprint(err)
				}
				if sides < 1 {
					sides = 1
				}
				if sides > 312 {
					sides = 312
				}
			}
			if inputlen > 1 {
				times, err = strconv.Atoi(args[1])
				if err != nil {
					return "failed to parse input: " + fmt.Sprint(err)
				}
				if times < 1 {
					times = 1
				} else if times > 312 {
					times = 312
				}
			}
			if inputlen > 2 {
				smode := args[2]
				if smode == "MOD" || smode == "AVG" {
					mode = smode
				}
			}
			ret := fmt.Sprintf("%vd%v %v result: ", times, sides, mode)
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			roll := func() int { return r.Intn(sides) + 1 }
			switch mode {
			case "SUM":
				sum := 0
				for i := 0; i < times; i++ {
					sum += roll()
				}
				ret += fmt.Sprint(sum)
			case "AVG":
				sum := 0
				for i := 0; i < times; i++ {
					sum += roll()
				}
				ret += fmt.Sprint(sum * 1.0 / times)
			case "MOD":
				sum := 0
				for i := 0; i < times; i++ {
					sum += roll()
				}
				ret += fmt.Sprint(sum % sides)
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "status_bind",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!server]",
		Desc:      "bind dynamic embed for server status to this channel",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			srv := args[0]
			repmsg := reply(session, message, "here be embed", DEL_NEVER)
			chn := message.ChannelID
			msg := repmsg.ID
			_, ok := known_servers[srv]
			if !ok {
				return "no such known server"
			}
			if !bind_server_embed(srv, chn, msg) {
				delmessage(session, repmsg)
				return "failed to bind embed"
			}
			return "OK"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "status_unbind",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!server]",
		Desc:      "unbind dynamic embed for server status from this channel",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			srv := args[0]
			chn := message.ChannelID
			_, ok := known_servers[srv]
			if !ok {
				return "no such known server"
			}
			if !unbind_server_embed(srv, chn) {
				return "failed to unbind embed"
			}
			return "OK"
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "promote",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!ckey]",
		Desc:      "promotes user to moderator",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ok, msg := add_moderator(args[0])
			if ok {
				return "OK, " + msg
			}
			return "FAIL, " + msg
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "demote",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!ckey]",
		Desc:      "demotes user from moderator",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {
			ok, msg := remove_moderator(args[0])
			if ok {
				return "OK, " + msg
			}
			return "FAIL, " + msg
		},
	})
	// ------------
	// ------------
	Register_command(&Dcommand{
		Command:   "list_moderators",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "list current moderators",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) (ret string) {
			defer logging_recover("dc_l_m")
			defer recovering_callback(func() { ret = "db request fail" })
			var ckey string
			db_template("select_moderators").query().parse(func() { ret += " `" + ckey + "`" }, &ckey)
			return "Current moderators: " + ret
		},
	})
	// ------------
}

// --------------------------------------------------------------------
/*
Dcommand register template below
	// ------------
	Register_command(&Dcommand{
		Command:   "",
		Minargs:   ,
		Permlevel: ,
		Usage:     "",
		Desc:      "",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string, server string) string {

		},
	})
	// ------------
	additional params:
		Temporary: ,
		Server_specific: ,
	// ------------
*/
// --------------------------------------------------------------------
