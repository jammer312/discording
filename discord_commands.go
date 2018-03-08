package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"sort"
	"strconv"
	"strings"
)

type dcfunc func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string

type Dcommand struct {
	Command    string
	Minargs    int
	Permlevel  int
	Usage      string
	Desc       string
	functional dcfunc
}

var Known_commands map[string]Dcommand

func Register_command(in Dcommand) {
	Known_commands[in.Command] = in
}

func (d *Dcommand) Exec(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
	return d.functional(session, message, args)
}
func (d *Dcommand) Usagestr() string {
	return Discord_command_character + d.Command + " " + d.Usage
}

func init() {
	Known_commands = make(map[string]Dcommand)
	// ------------
	Register_command(Dcommand{
		Command:   "check_permission",
		Minargs:   2,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "[!ckey] [!permlevel]",
		Desc:      "check permissions for user with supplied ckey for specified permissions level",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			ckey := args[0]
			permlevel, err := strconv.Atoi(args[1])
			if err != nil {
				return "error parsing permlevel argument: " + fmt.Sprint(err)
			}
			userid := ""
			for id, ck := range local_users {
				if strings.ToLower(ckey) == strings.ToLower(ck) {
					userid = id
					break
				}
			}
			if userid == "" {
				return "no user bound to that ckey: `" + ckey + "`"
			}
			user, err := session.User(userid)
			if err != nil {
				log.Println(err)
				return "failed to retrieve userid"
			}
			if Permissions_check(user, permlevel) {
				return "permission check for `" + ckey + "` at permlevel " + fmt.Sprint(permlevel) + " OK"
			} else {
				return "permission check for `" + ckey + "` at permlevel " + fmt.Sprint(permlevel) + " FAIL"
			}
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "list_admins",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "list known admin ckeys",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			ret := "known admins:\n"
			for _, admin := range Known_admins {
				ret += admin + "\n"
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "reload_admins",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "",
		Desc:      "sync admins list with hub",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			Load_admins(&Known_admins)
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "login",
		Minargs:   0,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "",
		Desc:      "receive channel permissions according to your ckey rank",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
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
	Register_command(Dcommand{
		Command:   "logoff",
		Minargs:   0,
		Permlevel: PERMISSIONS_REGISTERED,
		Usage:     "",
		Desc:      "remove channel permissions according to your ckey rank",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
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
	Register_command(Dcommand{
		Command:   "whoami",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "printout ckey you account is linked to",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			ckey := local_users[message.Author.ID]
			if ckey == "" {
				return "you're not registered"
			}
			return "you're registered as " + ckey
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "register",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "(re)bind your discord account to byond ckey",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
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
	Register_command(Dcommand{
		Command:   "list_registered",
		Minargs:   0,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "",
		Desc:      "list registered users in format [discord nick] -> [ckey]",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			rep := "registered users:\n"
			for login, ckey := range local_users {
				var nl string
				usr, err := session.User(login)
				if err != nil {
					Discord_message_send("debug", "ERR:", "", fmt.Sprint(err))
					nl = ""
				} else {
					nl = usr.String()
				}
				rep += fmt.Sprintf("%s -> %s\n", nl, ckey)
			}
			return rep
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "who",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "list players currently on server",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			br := Byond_query("who", false)
			return br.String()
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "channel_here",
		Minargs:   1,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[!channel type]",
		Desc:      "create and/or bind provided channel type to discord channel",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if Update_known_channel(args[0], message.ChannelID, guild) {
				return "changed `" + Dweaksanitize(args[0]) + "` channel to <#" + message.ChannelID + ">"
			} else {
				return "failed to change `" + Dweaksanitize(args[0]) + "` channel to <#" + message.ChannelID + ">"
			}
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "channel_list",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "",
		Desc:      "list known channel types and channels they're bound to",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			return List_known_channels()
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "channel_remove",
		Minargs:   0,
		Permlevel: PERMISSIONS_SUPERUSER,
		Usage:     "[?channel_type]",
		Desc:      "unbind either provided channel type or else one bound to receiving discord channel and forget about it",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			guild := Get_guild(session, message)
			if guild == "" {
				return "failed to retrieve guild"
			}
			if len(args) < 1 {
				tch := known_channels_id_t[message.ChannelID]
				if tch == "" {
					return "no channel bound here"
				}
				args = append(args, tch)
			}
			if Remove_known_channels(args[0], guild) {
				return "removed `" + Dweaksanitize(args[0]) + "`"
			} else {
				return "failed to remove `" + Dweaksanitize(args[0]) + "`"
			}

		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "ah",
		Minargs:   2,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "[!ckey] [!message]",
		Desc:      "sends adminPM containing [!message] to [!ckey]",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			Byond_query("adminhelp&admin="+Bquery_convert(local_users[message.Author.ID])+"&ckey="+Bquery_convert(args[0])+"&response="+Bquery_convert(strings.Join(args[1:], " ")), true)
			return ""
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "toggle_ooc",
		Minargs:   0,
		Permlevel: PERMISSIONS_ADMIN,
		Usage:     "",
		Desc:      "globally toggle ooc",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			Byond_query("OOC", true)
			return "toggled global OOC"
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "help",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "print list of commands available to you",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			call, creg, cadm, csup := make([]string, 0), make([]string, 0), make([]string, 0), make([]string, 0)
			ret := ""
			user := message.Author
			for comm, dcomm := range Known_commands {
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
			if Permissions_check(user, PERMISSIONS_NONE) {
				ret += "\n**Generic commands:**\n" + strings.Join(call, "\n")
			}
			if Permissions_check(user, PERMISSIONS_REGISTERED) {
				ret += "\n**Commands, available to registered users:**\n" + strings.Join(creg, "\n")
			}
			if Permissions_check(user, PERMISSIONS_ADMIN) {
				ret += "\n**Admin commands:**\n" + strings.Join(cadm, "\n")
			}
			if Permissions_check(user, PERMISSIONS_SUPERUSER) {
				ret += "\n**Superuser commands:**\n" + strings.Join(csup, "\n")
			}
			return ret
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "usage",
		Minargs:   1,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "[!cmd_name]",
		Desc:      "print description for provided command",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			cmd_name := args[0]
			dcmd, ok := Known_commands[cmd_name]
			if !ok {
				return "no such command"
			}
			if !Permissions_check(message.Author, dcmd.Permlevel) {
				return "missing required permissions"
			}
			return dcmd.Usagestr() + "\n" + dcmd.Desc
		},
	})
	// ------------
	// ------------
	Register_command(Dcommand{
		Command:   "adminwho",
		Minargs:   0,
		Permlevel: PERMISSIONS_NONE,
		Usage:     "",
		Desc:      "prints admins currently on server",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {
			br := Byond_query("adminwho", false)
			return br.String()
		},
	})
	// ------------

}

// --------------------------------------------------------------------
/*
Dcommand register template below
	// ------------
	Register_command(Dcommand{
		Command:   "",
		Minargs:   ,
		Permlevel: ,
		Usage:     "",
		Desc:      "",
		functional: func(session *discordgo.Session, message *discordgo.MessageCreate, args []string) string {

		},
	})
	// ------------
*/
// --------------------------------------------------------------------
