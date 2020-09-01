package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/grokify/html-strip-tags-go"
	"html"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	//server-invariant
	discord_bot_user_id       string //for permchecks
	discord_bot_token         string
	Discord_command_character string
	discord_superuser_id      string
	local_users               map[string]string  //user id -> ckey
	local_moderators          []string           //ckeys
	discord_player_roles      map[string]string  //guild id -> role
	known_channels_id_t       map[string]channel //channel id -> channel
	discord_spam_prot_bans    map[string]bool
	discord_spam_prot_checks  map[string]int //user id -> spammability
	discord_spam_prot_limit   int            //if checks amount exceed limit, autoban dat spammer scum
	discord_spam_prot_tick    int            //each tick all entries are nullified, in seconds

	//server-specific
	Known_admins                  map[string][]string            //server -> ckeys
	known_channels_s_t_id_m       map[string]map[string][]string //server -> type -> channel ids
	discord_subscriber_roles      map[string]map[string]string   //guild id -> server -> role
	discord_admin_roles           map[string]map[string]string   //guild id -> server -> role
	discord_onetime_subscriptions map[string]map[string]string   //guild id -> server -> users slap string
	discord_ahelp_locks           map[string]map[string]string   //server -> adminid -> ckey
)

type channel struct {
	generic_type string //ooc, admin, debug etc
	server       string //which server it belongs to
}

const (
	PERMISSIONS_NONE = iota - 1
	PERMISSIONS_REGISTERED
	PERMISSIONS_ADMIN
	PERMISSIONS_MODERATOR
	PERMISSIONS_SUPERUSER
)

const PERMISSIONS_SPECIAL = -1

const (
	MODERATOR_BAN_PERMISSION = PERMISSIONS_SUPERUSER
)

const (
	ROLE_PLAYER     = "player"
	ROLE_ADMIN      = "admin"
	ROLE_SUBSCRIBER = "subscriber"
)

const (
	BANTYPE_OOC = 1 << iota
	BANTYPE_COMMANDS
	BANTYPE_SPAM
)
const (
	BANSTRING_OOC      = "OOC"
	BANSTRING_COMMANDS = "COMMANDS"
)

//DEL_NEVER_MOD is not spamproof, so should rarely be used
const (
	DEL_NEVER_MOD  = -2
	DEL_NEVER      = -1
	DEL_DEFAULT    =  0
	DEL_LONG       =  3
	DEL_EXTRA_LONG = 10
)

const max_message_size = 2000
const message_mod_delay = 1;

type dban struct {
	reason    string
	admin     string
	bantype   int
	permlevel int
}

var known_bans_summary map[string]map[int]int
var known_ban_overrides map[string]map[string]map[int]int //server->ckey->permission->type 

var dsession, _ = discordgo.New()
var last_ahelp map[string]string

var emoji_stripper *regexp.Regexp
var url_embedder *regexp.Regexp
var channel_message_send_loops_online map[string]bool
var channel_buffers map[string][]string

func discord_init() {
	populate_configs()
	discord_bot_token = get_config_must("discord_bot_token")
	dsession.Token = discord_bot_token

	Discord_command_character = get_config_must("discord_command_character")
	discord_superuser_id = get_config_must("discord_superuser_id")
	discord_spam_prot_limit_str := get_config_must("discord_spam_prot_limit")
	var err error
	discord_spam_prot_limit, err = strconv.Atoi(discord_spam_prot_limit_str)
	if err != nil {
		log.Fatalln("Failed to parse 'discord_spam_prot_limit'")
	}
	discord_spam_prot_tick_str := get_config_must("discord_spam_prot_tick")
	discord_spam_prot_tick, err = strconv.Atoi(discord_spam_prot_tick_str)
	if err != nil {
		log.Fatalln("Failed to parse 'discord_spam_prot_tick'")
	}
	emoji_stripper = regexp.MustCompile("<a?:.+?:[0-9]{18}?>")
	url_embedder = regexp.MustCompile("https?://[\\w.]+/?[^ ]+")
	local_users = make(map[string]string)
	local_moderators = make([]string, 0)
	discord_player_roles = make(map[string]string)
	known_channels_id_t = make(map[string]channel)
	Known_admins = make(map[string][]string)
	known_channels_s_t_id_m = make(map[string]map[string][]string)
	discord_subscriber_roles = make(map[string]map[string]string)
	discord_admin_roles = make(map[string]map[string]string)
	discord_onetime_subscriptions = make(map[string]map[string]string)
	known_bans_summary = make(map[string]map[int]int)
	known_ban_overrides = make(map[string]map[string]map[int]int)
	last_ahelp = make(map[string]string)
	discord_spam_prot_checks = make(map[string]int)
	discord_spam_prot_bans = make(map[string]bool)
	discord_ahelp_locks = make(map[string]map[string]string)

	channel_message_send_loops_online = make(map[string]bool)
	channel_buffers = make(map[string][]string)
}

func reply(session *discordgo.Session, message *discordgo.MessageCreate, msg string, temporary int) []*discordgo.Message {
	if temporary == DEL_NEVER_MOD {
		rep, err := dsession.ChannelMessageSend(message.ChannelID, "<@!"+message.Author.ID+">, here will be result of your command")
		noerror(err)
		time.Sleep(time.Duration(message_mod_delay) * time.Second)
		dsession.ChannelMessageEdit(rep.ChannelID, rep.ID, "<@!"+message.Author.ID+">, "+msg)
		return []*discordgo.Message{rep}
	}
	rep := send_message_big(message.ChannelID, "<@!"+message.Author.ID+">, "+msg)
	if temporary == DEL_NEVER {
		return rep
	}
	if temporary == DEL_DEFAULT {
		temporary = 1
	}
	temporary = temporary * int(math.Ceil(math.Sqrt(2+float64(len(msg))/10)))
	if !is_in_private_channel(session, message) {
		batch_delete_in(session, rep, temporary)
	}
	return rep
}

func is_in_private_channel(session *discordgo.Session, message *discordgo.MessageCreate) bool {
	channel, err := session.State.Channel(message.ChannelID)
	if err != nil {
		log.Println("ERROR: failed to retrieve channel: ", err)
		return false
	}
	return channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM
}

func delete_in(session *discordgo.Session, message *discordgo.Message, seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
	err := session.ChannelMessageDelete(message.ChannelID, message.ID)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to delete reply message in discord: ", err)
	}
}

func batch_delete_in(session *discordgo.Session, messages []*discordgo.Message, seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
	for _, message := range(messages) {
		err := session.ChannelMessageDelete(message.ChannelID, message.ID)
		if err != nil {
			log.Println("NON-PANIC ERROR: failed to delete reply message in discord: ", err)
		}
	}
}

func delcommand(session *discordgo.Session, message *discordgo.MessageCreate) {
	if is_in_private_channel(session, message) {
		return
	}
	err := session.ChannelMessageDelete(message.ChannelID, message.ID)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to delete command message in discord: ", err)
	}
}

func delmessage(session *discordgo.Session, message *discordgo.Message) {
	err := session.ChannelMessageDelete(message.ChannelID, message.ID)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to delete message in discord: ", err)
	}
}

func Get_guild(session *discordgo.Session, message *discordgo.MessageCreate) string {
	channel, err := session.Channel(message.ChannelID)
	if err != nil {
		log.Println("Shiet: ", err)
		return ""
	}
	return channel.GuildID
}

func ckey_simplifier(s string) string {
	return strings.ToLower(strings.Replace(strings.Replace(s, ".", "", -1), "_", "", -1))
}

func get_permission_level_ckey(ckey, server string) int {
	if check_moderator(ckey) {
		return PERMISSIONS_MODERATOR;
	}
	if server != "" {
		asl, ok := Known_admins[server]
		if !ok {
			return PERMISSIONS_REGISTERED
		}
		for _, ackey := range asl {
			if ckey == ackey {
				return PERMISSIONS_ADMIN //this server admin
			}
		}
		return PERMISSIONS_REGISTERED
	}
	//no server, wide check
	for _, adminsl := range Known_admins {
		for _, ackey := range adminsl {
			if ckey == ackey {
				return PERMISSIONS_ADMIN //generic admin
			}
		}
	}
	return PERMISSIONS_REGISTERED
}
func get_permission_level_ckey_sp(ckey, server string) int {
	su, ok := local_users[discord_superuser_id]
	if ok && ckey_simplifier(su) == ckey {
		return PERMISSIONS_SUPERUSER
	}
	return get_permission_level_ckey(ckey, server)
}
func get_permission_level(user *discordgo.User, server string) int {
	if user.ID == discord_superuser_id || user.ID == discord_bot_user_id {
		return PERMISSIONS_SUPERUSER //bot admin
	}
	ckey := local_users[user.ID]
	if ckey == "" {
		return PERMISSIONS_NONE //not registered
	}

	ckey = ckey_simplifier(ckey)
	return get_permission_level_ckey(ckey, server)
}

func Permissions_check(user *discordgo.User, permission_level int, server string) bool {
	return get_permission_level(user, server) >= permission_level
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID {
		return
	}
	mcontent := message.ContentWithMentionsReplaced()
	if is_in_private_channel(session, message) && !Permissions_check(message.Author, PERMISSIONS_SUPERUSER, "") {
		reply(session, message, "FORBIDDEN, won't execute commands in private channels", DEL_NEVER)
		return
	}
	if len(mcontent) < 1 { //wut?
		return
	}
	if mcontent[:1] == Discord_command_character {
		if !spam_check(message.Author.ID) {
			delete_in(session, message.Message, 1)
			return
		}
		if len(mcontent) < 2 { //one for command char and at least one for command
			return
		}
		//it's command
		defer delcommand(session, message)
		var server string
		srvstr, ok := known_channels_id_t[message.ChannelID]
		if ok {
			server = srvstr.server
		}
		args := strings.Fields(mcontent[1:])
		command := strings.ToLower(args[0])
		if check_bans(message.Author, server, BANTYPE_COMMANDS) && command != "baninfo" {
			reply(session, message, "you're banned from this action. Try !baninfo.\nHead here -> "+config_entries["ban_unban_place"]+" <- if you feel you should be unbanned and **can prove it**", DEL_DEFAULT)
			return
		}
		if len(args) > 1 {
			args = args[1:]
		} else {
			args = make([]string, 0) //empty slice
		}
		dcomm, ok := known_commands[command]
		if !ok {
			reply(session, message, "unknown command: `"+Dweaksanitize(command)+"`", DEL_DEFAULT)
			return
		}
		log_line(fmt.Sprintf("<@%v>(%v)[%v]->`%v`", message.Author.ID, local_users[message.Author.ID], server, message.Content), "commands")
		if server == "" && dcomm.Server_specific {
			reply(session, message, "this command requires channel to be bound to server", DEL_DEFAULT)
			return
		}
		if !Permissions_check(message.Author, dcomm.Permlevel, server) {
			reply(session, message, "missing permissions required to run this command: `"+Dweaksanitize(command)+"`", DEL_DEFAULT)
			return
		}
		if len(args) < dcomm.Minargs {
			reply(session, message, "usage: "+dcomm.Usagestr(), DEL_LONG)
			return
		}
		ret := dcomm.Exec(session, message, args, server)
		if ret == "" {
			return
		}
		reply(session, message, ret, dcomm.Temporary)
		return
	} else if dsch := get_config("discord_shell_character"); dsch != "" && mcontent[:1] == dsch {
		if !spam_check(message.Author.ID) {
			delete_in(session, message.Message, 1)
			return
		}
		if len(mcontent) < 2 { //one for command char and at least one for command
			return
		}
		defer delete_in(session, message.Message, 1)
		udata := userdata{key: local_users[message.Author.ID], message: message, session: session}
		srvstr, ok := known_channels_id_t[message.ChannelID]
		if ok {
			udata.server = srvstr.server
		}
		if check_bans(message.Author, udata.server, BANTYPE_COMMANDS) {
			reply(session, message, "you're banned from this action. Try !baninfo.\nHead here -> "+config_entries["ban_unban_place"]+" <- if you feel you should be unbanned and **can prove it**", DEL_DEFAULT)
			return
		}
		log_line(fmt.Sprintf("<@%v>(%v)[%v]->`%v`", message.Author.ID, udata.key, udata.server, message.Content), "shell_commands")
		ret := shell_handler(udata, mcontent[1:])
		//TODO: make it temporary
		if udata.key == "" {
			udata.key = "nokey"
		}
		if udata.server == "" {
			udata.server = "noserver"
		}
		inpsplit := strings.Split(mcontent, "\n")
		inpjoined := strings.Join(inpsplit, "\n>	")
		send_message(message.ChannelID, fmt.Sprintf("```%s[%s]@%s:%s```\n%s", udata.key, message.Author.Username, udata.server, inpjoined, ret))
	}

	if known_channels_id_t[message.ChannelID].generic_type != "ooc" && known_channels_id_t[message.ChannelID].generic_type != "admin" {
		return
	}
	if !spam_check(message.Author.ID) {
		delete_in(session, message.Message, 0)
		return
	}
	shown_nick := local_users[message.Author.ID]
	defer delcommand(session, message)
	if shown_nick == "" {
		channel, err := session.Channel(message.ChannelID)
		if err != nil {
			log.Println("Shiet: ", err)
			reply(session, message, "failed to retrieve channel", DEL_DEFAULT)
		}
		if logoff_user(channel.GuildID, message.Author.ID) {
			reply(session, message, "you were logged off because of missing registration entry (try !register)", DEL_NEVER)
			return
		} else {
			reply(session, message, "no registration entry found + automatic logoff failed (probably because broken permissions); your message weren't delivered, register (!register) to be able to use OOC; also ask guild admins to fix bot's permissions", DEL_NEVER)
			return
		}
	}
	addstr := ""
	srv := known_channels_id_t[message.ChannelID]
	mcontent = emoji_stripper.ReplaceAllString(mcontent, "")
	var byondmcontent string //sent to byond
	isadminhere := Permissions_check(message.Author, PERMISSIONS_ADMIN, srv.server)
	if isadminhere {
		byondmcontent = "<font color='#39034f'>" + mcontent + "</font>"
		addstr = "&isadmin=1"
	} else {
		byondmcontent = strings.Replace(mcontent, "\n", "#", -1)
		byondmcontent = html.EscapeString(byondmcontent)
	}
	switch srv.generic_type {
	case "ooc":
		if check_bans(message.Author, srv.server, BANTYPE_OOC) {
			reply(session, message, "you're banned from this action. Try !baninfo", DEL_DEFAULT)
			return
		}
		br := Byond_query(srv.server, "ckey="+Bquery_convert(shown_nick, srv.server)+"&ooc="+Bquery_convert(byondmcontent, srv.server)+addstr, true)
		if br.String() == "muted" {
			reply(session, message, "your ckey is muted from OOC", DEL_DEFAULT)
			return
		}
		if br.String() == "globally muted" {
			reply(session, message, "OOC is globally muted", DEL_DEFAULT)
			return
		}
		Discord_message_send(srv.server, "ooc", "DOOC: "+shown_nick, strip.StripTags(mcontent))
	case "admin":
		if !isadminhere {
			reply(session, message, "You have no privilegies to write here", DEL_DEFAULT)
			return //not today
		}
		Byond_query(srv.server, "admin="+Bquery_convert(shown_nick, srv.server)+"&asay="+Bquery_convert(byondmcontent, srv.server), true)
		Discord_message_send(srv.server, "admin", "DASAY: "+shown_nick, strip.StripTags(mcontent))
	default:
	}
}

//this one uses loops to try to handle spam
//it hangs on attempting to send message if rate limit is exceeded, so it queues more chunks
var send_message_mutex sync.Mutex

func send_message(channel, message string) {
	message = fmt.Sprintf("`[%v]` %v", get_time(), message)
	send_message_mutex.Lock()
	if channel_buffers[channel] == nil {
		channel_buffers[channel] = make([]string, 1)
		channel_buffers[channel][0] = message
	} else {
		channel_buffers[channel] = append(channel_buffers[channel], message)
	}
	if channel_message_send_loops_online[channel] {
		send_message_mutex.Unlock() //doublelol
		return
	}
	channel_message_send_loops_online[channel] = true
	send_message_mutex.Unlock() //lol
	go func() {
		for {
			send_message_mutex.Lock()
			if len(channel_buffers[channel]) <= 0 {
				send_message_mutex.Unlock()
				break
			}
			condensed := strings.Join(channel_buffers[channel], "\n")
			var chunk string
			if len(condensed) <= max_message_size {
				chunk = condensed
				channel_buffers[channel] = nil
			} else {
				li := strings.LastIndexByte(condensed[:max_message_size], '\n')
				if li < 0 {
					//channel_message_send_loops_online[channel] = false
					//panic("send message WTF li<0: `" + condensed[:max_message_size] + "`")
					//some shitter sent VERY long line from server, force cut it
					li = max_message_size - 5
					chunk = condensed[:li] + "**#**"
					channel_buffers[channel] = []string{"**#**" + condensed[li:]}
				} else {
					chunk = condensed[:li]
					channel_buffers[channel] = []string{condensed[li+1:]}
				}
			}
			send_message_mutex.Unlock()
			_, err := dsession.ChannelMessageSend(channel, chunk)
			if err != nil {
				log.Println("DERR: failed to send message to discord: ", err)
			}
		}
		send_message_mutex.Lock()
		channel_message_send_loops_online[channel] = false
		send_message_mutex.Unlock()
	}()
}

func send_message_big(channel, message string) []*discordgo.Message {
	ret := make([]*discordgo.Message,0)
	for len(message) > 0 {
		var tosend string
		if len(message)>max_message_size {
			li := strings.LastIndexByte(message[:max_message_size],'\n')
			if li < 0 {
				li = max_message_size-5
				tosend=message[:li]+"**#**"
				message="**#**"+message[li:]
			} else {
				tosend=message[:li]
				message=message[li:]
			}
		} else {
			tosend=message
			message=""
		}
		rep, err := dsession.ChannelMessageSend(channel, tosend)
		if err != nil {
			log.Println("DERR: failed to send message to discord: ", err)
			break
		} else {
			ret=append(ret, rep)
		}
	}
	return ret
}

func Discord_subsriber_message_send(servername, channel, message string) {
	defer logging_recover("Dsms")
	srvchans, ok := known_channels_s_t_id_m[servername]
	if !ok {
		panic("unknown server, " + servername)
	}
	channels, ok := srvchans[channel]
	if !ok || len(channels) < 1 {
		return
	}
	flush_onetime_subscriptions(servername)
	for _, id := range channels {
		chann, cerr := dsession.Channel(id)
		if cerr != nil {
			log.Println("ERROR: CHAN<-ID FAIL: ", cerr)
			continue
		}
		guild, gerr := dsession.Guild(chann.GuildID)
		if gerr != nil {
			log.Println("ERROR: GUILD<-ID FAIL: ", gerr)
			continue
		}
		var rid string
		guildsubrole, ok := discord_subscriber_roles[guild.ID]
		if ok {
			rid, ok = guildsubrole[servername]
			if !ok {
				rid = ""
			} else {
				rid = "<@&" + rid + ">, "
			}
		}
		var subs string
		guildoncesubs, ok := discord_onetime_subscriptions[guild.ID]
		if ok {
			subs, ok = guildoncesubs[servername]
			if !ok {
				subs = ""
			} else {
				subs += ", "
			}
		}
		send_message(id, rid+subs+Dsanitize(message))
	}
}
func Discord_message_send(servername, channel, boldprefix, message string) {
	defer logging_recover("Dms")
	srvchans, ok := known_channels_s_t_id_m[servername]
	if !ok {
		panic("unknown server, " + servername)
	}
	channels, ok := srvchans[channel]
	if !ok || len(channels) < 1 {
		return //no bound channels
	}
	for _, id := range channels {
		send_message(id, "**"+Dsanitize(boldprefix)+":** "+Dsanitize(message))
	}
}
func Discord_message_send_raw(servername, channel, message string) {
	defer logging_recover("Dmsr")
	srvchans, ok := known_channels_s_t_id_m[servername]
	if !ok {
		panic("unknown server, " + servername)
	}
	channels, ok := srvchans[channel]
	if !ok || len(channels) < 1 {
		return //no bound channels
	}
	for _, id := range channels {
		send_message(id, message)
	}
}

func Discord_send_embed(servername, channel string, embed *discordgo.MessageEmbed) {
	defer logging_recover("Dse")
	srvchans, ok := known_channels_s_t_id_m[servername]
	if !ok {
		panic("unknown server, " + servername)
	}
	channels, ok := srvchans[channel]
	if !ok || len(channels) < 1 {
		return //no bound channels
	}
	for _, id := range channels {
		_, err := dsession.ChannelMessageSendEmbed(id, embed)
		if err != nil {
			log.Println("DERR: failed to send embed to discord: ", err)
		}
	}
}

func Discord_replace_embed(channelid, messageid string, embed *discordgo.MessageEmbed) (ret bool) {
	defer logging_recover("Dre")
	_, err := dsession.ChannelMessageEditComplex(discordgo.NewMessageEdit(channelid, messageid).SetContent(fmt.Sprintf("Last status update: %v UTC+3(Moscow)", get_time())).SetEmbed(embed))
	if fmt.Sprint(err) == "HTTP 404 Not Found, {\"code\": 10003, \"message\": \"Unknown Channel\"}" {
		return false
	}
	ret = true
	noerror(err)
	return
}

func Discord_message_propagate(servername, channel, prefix, ckey, message, chanid string) {
	//given channel id and other params sends message to all channels except specified one
	defer logging_recover("Dmp")
	srvchans, ok := known_channels_s_t_id_m[servername]
	if !ok {
		panic("unknown server, " + servername)
	}
	channels, ok := srvchans[channel]
	if !ok || len(channels) < 1 {
		return //no bound channels
	}
	var delim string
	if prefix != "" && ckey != "" {
		delim = " "
	}
	for _, id := range channels {
		if id == chanid {
			continue
		}
		send_message(id, "**"+Dsanitize(prefix+delim+ckey)+":** "+Dsanitize(message))
	}
}

func Discord_private_message_send(user *discordgo.User, message string) bool {
	channel, err := dsession.UserChannelCreate(user.ID)
	if err != nil {
		log.Println("Failed to create private channel: ", err)
		return false
	}
	send_message(channel.ID, message)
	return true
}

func Dsanitize(m string) string {
	out := strings.Replace(m, "\\", "\\\\", -1)
	out = strings.Replace(out, "*", "\\*", -1)
	out = strings.Replace(out, "`", "\\`", -1)
	out = strings.Replace(out, "_", "\\_", -1)
	out = strings.Replace(out, "~", "\\~", -1)
	out = strings.Replace(out, "@everyone", "[я долбоеб]", -1)
	out = strings.Replace(out, "@here", "[я долбоеб]", -1)
	out = strings.Replace(out, "@", "\\@", -1)
	out = url_embedder.ReplaceAllString(out, "<$0>")
	return out
}

func Dweaksanitize(m string) string {
	out := strings.Replace(m, "`", "\\`", -1)
	return out
}

func trim(str string) string {
	return strings.Trim(str, " ")
}

func populate_known_channels() {
	defer logging_recover("pkc")
	for k := range known_channels_id_t {
		delete(known_channels_id_t, k)
	} //clear id->type pairs
	for k, v := range known_channels_s_t_id_m {
		for sk := range v {
			delete(v, sk)
		}
		delete(known_channels_s_t_id_m, k)
	} //clear server->type->ids and server->type
	var ch, id, srv string
	clcllb := func() {
		known_channels_id_t[id] = channel{ch, srv}
		if srv == "" {
			log.Println("DB: setting `" + id + "` to '" + srv + "@" + ch + "';")
			return
		}
		srvchans, ok := known_channels_s_t_id_m[srv]
		if !ok {
			srvchans = make(map[string][]string)
		}
		chsl, ok := srvchans[ch]
		if !ok {
			chsl = make([]string, 0)
		}
		srvchans[ch] = append(chsl, id)
		known_channels_s_t_id_m[srv] = srvchans // not needed since maps are references, but it's nice for readability
		log.Println("DB: setting `" + id + "` to '" + srv + "@" + ch + "';")
	}
	db_template("select_known_channels").query().parse(clcllb, &ch, &id, &srv)
}

func add_known_channel(srv, t, id, gid string) bool {
	defer logging_recover("akc")
	if db_template("add_known_channel").exec(t, id, gid, srv).count() > 0 {
		populate_known_channels()
		return true
	}
	return false
}

func Remove_known_channels(srv, t, gid string) bool {
	defer logging_recover("rkc")
	var res *db_query_result
	if gid == "" {
		res = db_template("remove_known_channels").exec(t, srv)
	} else {
		res = db_template("remove_known_channels_guild").exec(t, gid, srv)
	}
	if res.count() > 0 {
		populate_known_channels()
		return true
	}
	return false
}

func List_known_channels() string {
	ret := "known channels:\n"
	for id, t := range known_channels_id_t {
		ret += fmt.Sprintf("<#%s> <-> `%s@%s`\n", id, t.server, t.generic_type)
	}
	return ret
}

func Update_known_channel(srv, t, id, gid string) bool {
	defer logging_recover("ukc")
	if db_template("update_known_channel").exec(t, id, gid, srv).count() > 0 {
		populate_known_channels() //update everything
		return true
	} else {
		return add_known_channel(srv, t, id, gid)
	}
}

func Remove_token(ttype, data string) bool {
	defer logging_recover("rt")
	if db_template("remove_token").exec(ttype, data).count() > 0 {
		return true
	}
	return false
}

func remove_token_by_id(id string) bool {
	defer logging_recover("rtbi")
	if db_template("remove_token_by_id").exec(id).count() > 0 {
		return true
	}
	return false
}

func Create_token(ttype, data string) string {
	defer logging_recover("ct")
	id := uuid.New().String()
	if db_template("create_token").exec(id, ttype, data).count() > 0 {
		return id
	}
	return ""
}

func expend_token(id string) (ttype, data string) {
	defer logging_recover("et")
	db_template("select_token").row(id).parse(&ttype, &data)
	remove_token_by_id(id)
	return
}

func Discord_process_token(id, ckey string) {
	ttype, data := expend_token(id)
	if ttype == "" {
		return
	}
	switch ttype {
	case "register":
		register_user(data, ckey)
	default:
	}
}

func deregister_user(login string) string {
	defer logging_recover("dru")
	if db_template("delete_user_did").exec(login).count() > 0 {
		delete(local_users, login)
		return "OK"
	}
	return "FAIL"
}

func register_user(login, ckey string) {
	defer logging_recover("ru")
	db_template("delete_user_did").exec(login)
	db_template("delete_user_ckey").exec(ckey)
	db_template("register_user").exec(login, ckey)
	update_local_user(login)
	user, err := dsession.User(login)
	if err != nil {
		panic("failed to get user")
	}
	Discord_private_message_send(user, "Registered as `"+ckey+"`")
}

func update_local_users() {
	defer logging_recover("ulus")
	for k := range local_users {
		delete(local_users, k)
	}
	var login, ckey string
	db_template("select_users").query().parse(func() {
		local_users[login] = ckey
	}, &login, &ckey)
}

func update_local_user(login string) (ckey string) {
	defer logging_recover("ulu")
	db_template("select_user").row(login).parse(&ckey)
	for l, c := range local_users {
		if l == login || c == ckey {
			delete(local_users, l)
		}
	}
	local_users[login] = ckey
	return
}

func populate_known_roles() {
	logging_recover("pkr")
	for k := range discord_player_roles {
		delete(discord_player_roles, k)
	}
	for k := range discord_admin_roles {
		delete(discord_admin_roles, k)
	}
	var gid, rid, tp, srv string
	closure_callback := func() {
		switch tp {
		case ROLE_PLAYER:
			discord_player_roles[gid] = rid
		case ROLE_ADMIN:
			if srv != "" {
				m, ok := discord_admin_roles[gid]
				if !ok {
					m = make(map[string]string)
					discord_admin_roles[gid] = m
				}
				m[srv] = rid
			}
		case ROLE_SUBSCRIBER:
			m, ok := discord_subscriber_roles[gid]
			if !ok {
				m = make(map[string]string)
				discord_subscriber_roles[gid] = m
			}
			m[srv] = rid
		}
	}
	db_template("select_known_roles").query().parse(closure_callback, &gid, &rid, &tp, &srv)
}
func update_known_role(gid, tp, rid, srv string) bool {
	defer logging_recover("ukr")
	if db_template("update_known_role").exec(rid, gid, tp, srv).count() > 0 {
		populate_known_roles()
		return true
	}
	return create_known_role(gid, tp, rid, srv)
}
func create_known_role(gid, tp, rid, srv string) bool {
	defer logging_recover("ckr")
	if db_template("create_known_role").exec(gid, rid, tp, srv).count() > 0 {
		populate_known_roles()
		return true
	}
	return false
}
func remove_known_role(gid, tp, srv string) bool {
	defer logging_recover("rkr")
	if db_template("remove_known_role").exec(gid, tp, srv).count() > 0 {
		populate_known_roles()
		return true
	}
	return false
}

func populate_bans() {
	defer logging_recover("pb")
	//clean known
	for k := range known_bans_summary {
		delete(known_bans_summary, k)
	}
	var ckey string
	var bantype, permission int
	closure_callback := func() {
		_, ok := known_bans_summary[ckey]
		if !ok {
			known_bans_summary[ckey] = make(map[int]int)
		}
		for i := permission - 1; i >= 0; i-- {
			known_bans_summary[ckey][i] |= bantype
		}
	}
	db_template("fetch_bans").query().parse(closure_callback, &ckey, &bantype, &permission)
}

func populate_ban_overrides() {
	defer logging_recover("pbo")
	for k := range known_ban_overrides {
		delete(known_ban_overrides, k)
	}
	var server, ckey string
	var bantype, permission int
	closure_callback := func() {
		srv, ok := known_ban_overrides[server]
		if !ok {
			srv = make(map[string]map[int]int)
			known_ban_overrides[server] = srv
		}
		usr, ok := known_ban_overrides[server][ckey]
		if !ok {
			usr = make(map[int]int)
			known_ban_overrides[server][ckey] = usr
		}

		for i := permission - 1; i >= 0; i-- {
			usr[i] |= bantype
		}
	}
	db_template("fetch_ban_overrides").query().parse(closure_callback, &server, &ckey, &bantype, &permission)
}

func update_ban(ckey, reason string, user *discordgo.User, tp int) (succ bool, msg string) {
	defer logging_recover("ub")
	ckey = ckey_simplifier(ckey)
	permissions := get_permission_level(user, "")
	var admin string
	if user.ID == dsession.State.User.ID {
		admin = dsession.State.User.Username
	} else {
		admin = local_users[user.ID]
	}
	if permissions < PERMISSIONS_ADMIN {
		return false, "missing permissions (how the fuck did you get there?)"
	}
	msg = "lr"
	if db_template("lookup_ban").exec(ckey, tp, admin).count() > 0 {
		msg = "ur"
		if db_template("update_ban").exec(reason, ckey, admin, tp, permissions).count() > 0 {
			populate_bans()
			return true, "updated"
		}
		return false, "some strange shit happened"
	} else {
		msg = "cr"
		if db_template("create_ban").exec(ckey, admin, reason, tp, permissions).count() > 0 {
			populate_bans()
			return true, "created"
		}
		return false, "some neat shit happened"
	}
}

func update_ban_override(server, ckey string, user *discordgo.User, tp int) (succ bool, msg string) {
	defer logging_recover("ubo")
	ckey = ckey_simplifier(ckey)
	permissions := get_permission_level(user, server)
	if permissions < PERMISSIONS_ADMIN {
		return false, "403 forbidden"
	}
	msg = "database failure"
	if db_template("check_ban_override").exec(server, ckey, tp, permissions).count() > 0 {
		populate_ban_overrides()
		return true, "stronger override is in effect"
	}
	if db_template("promote_ban_override").exec(server, ckey, tp, permissions).count() > 0 {
		populate_ban_overrides()
		return true, "promoted existing override"
	}
	succ = db_template("add_ban_override").exec(server, ckey, tp, permissions).count() > 0
	if succ {
		msg = "added new override"
	}
	return succ, msg
}

func remove_ban(ckey string, tp int, user *discordgo.User) (succ bool, msg string) {
	defer logging_recover("rb")
	ckey = ckey_simplifier(ckey)
	permissions := get_permission_level(user, "")
	admin := local_users[user.ID]
	if permissions < PERMISSIONS_ADMIN {
		return false, "missing permissions (how the fuck did you get there?)"
	}
	msg = "rr"
	cnt := db_template("remove_ban").exec(ckey, tp, permissions, admin).count()
	if cnt > 0 {
		populate_bans()
		return true, fmt.Sprintf("%v bans removed", cnt)
	}
	return false, "no bans removed"
}

func wipe_ban_overrides(server, ckey string, user *discordgo.User) (succ bool, msg string) {
	defer logging_recover("wbo")
	ckey = ckey_simplifier(ckey)
	permissions := get_permission_level(user, "")
	if permissions < PERMISSIONS_ADMIN {
		return false, "403 forbidden"
	}
	msg = "database failure"
	if cnt := db_template("wipe_ban_overrides").exec(server, ckey, permissions).count(); cnt > 0 {
		populate_ban_overrides()
		return true, fmt.Sprintf("%v bans overrides removed", cnt)
	}
	return false, "no ban overrides removed"
}

func check_bans(user *discordgo.User, server string, tp int) bool {
	ckey := local_users[user.ID]
	ckey = ckey_simplifier(ckey)
	if ckey == "" {
		return false
	}
	banarr, ok := known_bans_summary[ckey]
	if !ok {
		return false
	}
	ourperms := get_permission_level_ckey_sp(ckey, server)
	bant := banarr[ourperms]
	banned := (bant & tp) != 0
	srv_overrides, ok := known_ban_overrides[server]
	if ok {
		usr_overrides, ok := srv_overrides[ckey]
		if ok {
			unbant := usr_overrides[ourperms]
			banned = ((bant ^ (bant & unbant)) & tp) != 0
		}
	}
	return banned
}

func check_bans_readable(user *discordgo.User, server string, tp int) string {
	ckey := local_users[user.ID]
	ckey = ckey_simplifier(ckey)
	if ckey == "" {
		return ""
	}
	banarr, ok := known_bans_summary[ckey]
	if !ok {
		return ""
	}
	ourperms := get_permission_level_ckey_sp(ckey, server)
	bant := banarr[ourperms]
	bantype := make([]string, 0)
	if bant&BANTYPE_OOC != 0 {
		bantype = append(bantype, BANSTRING_OOC)
	}
	if bant&BANTYPE_COMMANDS != 0 {
		bantype = append(bantype, BANSTRING_COMMANDS)
	}
	bantypestring := strings.Join(bantype, ", ")
	return "Here you are banned from: " + bantypestring
}

func subscribe_user(guildid, userid, srv string) bool {
	var subscriber_role string
	var ok bool
	gsubs, ok := discord_subscriber_roles[guildid]
	if !ok {
		return false
	}
	subscriber_role, ok = gsubs[srv]
	if !ok {
		log.Println("Failed to find subscriber role for server " + srv)
		return false
	}
	err := dsession.GuildMemberRoleAdd(guildid, userid, subscriber_role)
	if err != nil {
		log.Println("Subscribe error: ", err)
		return false
	}
	return true
}
func unsubscribe_user(guildid, userid, srv string) bool {
	ckey := update_local_user(userid)
	if ckey == "" {
		return false
	}
	ckey = strings.ToLower(ckey)
	var subscriber_role string
	var ok bool
	gsubs, ok := discord_subscriber_roles[guildid]
	if !ok {
		return false
	}
	subscriber_role, ok = gsubs[srv]
	if !ok {
		log.Println("Failed to find subscriber role for server " + srv)
		return false
	}
	err := dsession.GuildMemberRoleRemove(guildid, userid, subscriber_role)
	if err != nil {
		log.Println("Subscribe error: ", err)
		return false
	}
	return true
}

func subscribe_user_once(guildid, userid, srv string) bool {
	defer logging_recover("suo")
	if db_template("select_onetime_sub").exec(userid, guildid, srv).count() == 0 {
		if db_template("create_onetime_sub").exec(userid, guildid, srv).count() == 0 {
			return false
		}
	}
	return true
}

func flush_onetime_subscriptions(servername string) {
	defer logging_recover("fos")
	for k, v := range discord_onetime_subscriptions {
		for l := range v {
			delete(v, l)
		}
		delete(discord_onetime_subscriptions, k)
	} //delete in any case
	var userid, guildid, srv string
	closure_callback := func() {
		_, ok := discord_onetime_subscriptions[guildid]
		if !ok {
			discord_onetime_subscriptions[guildid] = make(map[string]string)
		}
		gsubs := discord_onetime_subscriptions[guildid]
		crstr, ok := gsubs[srv]
		if !ok {
			crstr = ""
		} else {
			crstr += ", "
		}
		gsubs[srv] = crstr + "<@!" + userid + ">"
	}
	db_template("select_onetime_subs").query(servername).parse(closure_callback, &userid, &guildid, &srv)
	db_template("remove_onetime_subs").exec(servername)
}

func login_user(guildid, userid string) bool {
	ckey := update_local_user(userid)
	if ckey == "" {
		return false
	}
	ckey = strings.ToLower(ckey)
	var player_role string
	var ok bool
	player_role, ok = discord_player_roles[guildid]
	if !ok {
		return true
	}
	err := dsession.GuildMemberRoleAdd(guildid, userid, player_role)
	if err != nil {
		log.Println("Login error: ", err)
		return false
	}

	for server := range known_servers {
		isadmin := false
		adm_entry, ok := Known_admins[server]
		if !ok {
			continue
		}
		for _, admin := range adm_entry {
			if ckey == strings.ToLower(admin) {
				isadmin = true
				break
			}
		}
		if !isadmin {
			continue
		}
		var admin_role string
		gadms, ok := discord_admin_roles[guildid]
		if !ok {
			continue
		}
		admin_role, ok = gadms[server]
		if !ok {
			continue
		}
		err = dsession.GuildMemberRoleAdd(guildid, userid, admin_role)
		if err != nil {
			log.Println("Login error: ", err)
			return false
		}
	}

	return true
}

func logoff_user(guildid, userid string) bool {
	var player_role string
	var ok bool
	player_role, ok = discord_player_roles[guildid]
	if !ok {
		return true
	}
	err := dsession.GuildMemberRoleRemove(guildid, userid, player_role)
	if err != nil {
		log.Println("Logoff error: ", err)
		return false
	}
	for server := range known_servers {
		var admin_role string
		gadms, ok := discord_admin_roles[guildid]
		if !ok {
			continue
		}
		admin_role, ok = gadms[server]
		if !ok {
			continue
		}
		err = dsession.GuildMemberRoleRemove(guildid, userid, admin_role)
		if err != nil {
			log.Println("Logoff error: ", err)
			return false
		}
	}
	return true
}

func spam_check(userid string) bool {
	ccnt := discord_spam_prot_checks[userid]
	//	log.Println("check", ccnt)
	discord_spam_prot_checks[userid] = ccnt + 1
	if ccnt >= discord_spam_prot_limit && !discord_spam_prot_bans[userid] {
		ckey := local_users[userid]
		//		log.Println("banning")
		if ckey != "" {
			update_ban(ckey, "spam autoban", dsession.State.User, BANTYPE_OOC|BANTYPE_COMMANDS)
		}
		discord_spam_prot_bans[userid] = true
	}
	if discord_spam_prot_bans[userid] {
		return ccnt == 0
	}
	return true
}

const guildslim = 100
const userslim = 1000

//iterates over all guilds over all users, stripping excess roles
//could make it respect limits (so if there's more items than limit allows, make multiple requests) but nah
func update_roles() {
	defer logging_recover("u_r")
	guilds, err := dsession.UserGuilds(guildslim, "", "")
	noerror(err)
	for _, guild := range guilds {
		pl_role, pok := discord_player_roles[guild.ID] //here role
		adm_role, aok := discord_admin_roles[guild.ID] //here server->role
		adm_role_inv := make(map[string]string)
		if aok {
			for k, v := range adm_role {
				adm_role_inv[v] = k
			}
		}
		if !(pok || aok) {
			continue //no such roles here, nothing to strip
		}
		users, err := dsession.GuildMembers(guild.ID, "", guildslim)
		noerror(err)
		for _, user := range users {
			for _, role := range user.Roles {
				if pok && role == pl_role {
					if get_permission_level(user.User, "") < PERMISSIONS_REGISTERED {
						dsession.GuildMemberRoleRemove(guild.ID, user.User.ID, role)
						//I'd put noerror here but I'm afraid that fukken onyx circus will strike back
						//so simply log it
						log.Printf("stripping playerrole off %v because of missing registration", user.User.Username)
					}
				}
				if aok {
					srv, ok := adm_role_inv[role]
					if ok && get_permission_level(user.User, srv) < PERMISSIONS_ADMIN {
						dsession.GuildMemberRoleRemove(guild.ID, user.User.ID, role)
						//same here
						log.Printf("stripping %v adminrole off %v because he's not admin", srv, user.User.Username)
					}
				}
			}
		}
	}
}

func add_moderator(ckey string) (state bool, msg string) {
	defer logging_recover("a_m")
	ckey = ckey_simplifier(ckey)
	found := false
	for _, ck := range local_moderators {
		if ckey == ck {
			found = true
			break
		}
	}
	if found {
		return false, "user is already moderator"
	}
	local_moderators = append(local_moderators, ckey)
	msg = "strange db error"
	db_template("add_moderator").exec(ckey)
	return true, "successfully added moderator `" + ckey + "`"
}

func remove_moderator(ckey string) (state bool, msg string) {
	defer logging_recover("r_m")
	ckey = ckey_simplifier(ckey)
	found := false
	index := 0
	for i, ck := range local_moderators {
		if ckey == ck {
			found = true
			index = i
			break
		}
	}
	if !found {
		return false, "no such moderator"
	}
	local_moderators = append(local_moderators[:index], local_moderators[index+1:]...)
	msg = "strange db error"
	count := db_template("remove_moderator").exec(ckey).count()
	if count == 0 {
		return false, "for some reason db didn't contain records of that moderator"
	}
	return true, "successfully removed moderator `" + ckey + "`"
}

func check_moderator(ckey string) bool {
	ckey = ckey_simplifier(ckey)
	for _, ck := range local_moderators {
		if ck == ckey {
			return true
		}
	}
	return false
}

func initial_update_moderators() {
	var ckey string
	db_template("select_moderators").query().parse(func() { local_moderators = append(local_moderators, ckey) }, &ckey)
}

var spamticker, updateticker chan int

func set_status() {
	defer rise_error("s_s")
	noerror(dsession.UpdateStatus(0, "!info"))
}

func nickchange_guard(s *discordgo.Session, up *discordgo.GuildMemberUpdate) {
	if up.User.ID == dsession.State.User.ID {
		//it's our bot
		nick := config_entries[up.GuildID+"nick"]
		if up.Nick == "" {
			up.Nick = dsession.State.User.Username
		}
		if nick == "" {
			log_line("Nick changed in "+up.GuildID+" from "+nick+" to "+up.Nick+"", "nick_guard")
			return
		}
		if up.Nick != nick {
			log_line("Nick changed in "+up.GuildID+" from "+nick+" to "+up.Nick+"; reverting", "nick_guard")
			defer logging_recover("nickchange_guard")
			noerror(s.GuildMemberNickname(up.GuildID, "@me", nick))
		}
	}
}

func Dopen() {
	defer logging_crash("Do")
	var err error
	dsession.State.User, err = dsession.User("@me")
	noerror(err)
	discord_bot_user_id = dsession.State.User.ID
	err = dsession.Open()
	noerror(err)
	log.Print("Successfully connected to discord, now running as ", dsession.State.User)
	set_status()
	populate_known_channels()
	update_local_users()
	populate_known_roles()
	populate_bans()
	populate_ban_overrides()
	Load_admins()
	initial_update_moderators()
	spamticker = start_ticker(discord_spam_prot_tick, func() {
		for uid, _ := range discord_spam_prot_checks {
			discord_spam_prot_checks[uid] = 0
		}
	})
	dsession.AddHandler(messageCreate)
	dsession.AddHandler(nickchange_guard)
	ver := os.Getenv("HEROKU_RELEASE_VERSION")
	com := os.Getenv("HEROKU_SLUG_COMMIT")
	for _, srv := range known_servers {
		Discord_message_send_raw(srv.name, "bot_status", fmt.Sprintf("**BOT:** now running %v `%v`", ver, com))
	}
	update_roles()
	updateticker = start_ticker(30, func() {
		update_roles()
	})
}

func Dclose() {
	defer logging_crash("Dc")
	for _, srv := range known_servers {
		Discord_message_send(srv.name, "bot_status", "BOT", "shutting down (soft)")
	}
	stop_ticker(spamticker)
	stop_ticker(updateticker)
	err := dsession.Close()
	noerror(err)
}
