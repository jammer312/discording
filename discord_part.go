package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/grokify/html-strip-tags-go"
	"html"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	//server-invariant
	discord_bot_user_id       string //for permchecks
	discord_bot_token         string
	Discord_command_character string
	discord_superuser_id      string
	local_users               map[string]string  //user id -> ckey
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
	PERMISSIONS_SUPERUSER
)

const (
	ROLE_PLAYER     = "player"
	ROLE_ADMIN      = "admin"
	ROLE_SUBSCRIBER = "subscriber"
)

const (
	BANTYPE_OOC = 1 << iota
	BANTYPE_COMMANDS
)
const (
	BANSTRING_OOC      = "OOC"
	BANSTRING_COMMANDS = "COMMANDS"
)

const (
	DEL_NEVER   = -1
	DEL_DEFAULT = 0
	DEL_LONG    = 3
)

type dban struct {
	reason    string
	admin     string
	bantype   int
	permlevel int
}

var known_bans map[string]dban

var dsession, _ = discordgo.New()
var last_ahelp map[string]string

var emoji_stripper *regexp.Regexp

// db table app_config {key<->value}
var config_entries map[string]string

func populate_configs() {
	defer logging_recover("p_c")
	config_entries = make(map[string]string)
	rows, err := Database.Query("select KEY, VALUE from app_config;")
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var key, val string
		if err = rows.Scan(&key, &val); err != nil {
			panic(err)
		}
		config_entries[key] = val
	}
}

func check_config(entry string) bool {
	_, ok := config_entries[entry]
	return ok
}

func get_config(entry string) string {
	return config_entries[entry]
}

func get_config_must(entry string) string {
	val, ok := config_entries[entry]
	if !ok {
		panic("Failed to retrieve '" + entry + "'' config entry")
	}
	return val
}

func init() {
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
	local_users = make(map[string]string)
	discord_player_roles = make(map[string]string)
	known_channels_id_t = make(map[string]channel)
	Known_admins = make(map[string][]string)
	known_channels_s_t_id_m = make(map[string]map[string][]string)
	discord_subscriber_roles = make(map[string]map[string]string)
	discord_admin_roles = make(map[string]map[string]string)
	discord_onetime_subscriptions = make(map[string]map[string]string)
	known_bans = make(map[string]dban)
	last_ahelp = make(map[string]string)
	discord_spam_prot_checks = make(map[string]int)
	discord_spam_prot_bans = make(map[string]bool)
	discord_ahelp_locks = make(map[string]map[string]string)
}

func reply(session *discordgo.Session, message *discordgo.MessageCreate, msg string, temporary int) {
	rep, err := session.ChannelMessageSend(message.ChannelID, "<@!"+message.Author.ID+">, "+msg)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to send reply message to discord: ", err)
	}
	if temporary < 0 {
		return
	}
	if temporary == DEL_DEFAULT {
		temporary = 1
	}
	temporary = temporary * int(math.Ceil(math.Sqrt(2+float64(len(msg))/10)))
	if !is_in_private_channel(session, message) {
		go delete_in(session, rep, temporary)
	}
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

func delcommand(session *discordgo.Session, message *discordgo.MessageCreate) {
	if is_in_private_channel(session, message) {
		return
	}
	err := session.ChannelMessageDelete(message.ChannelID, message.ID)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to delete command message in discord: ", err)
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

func ckey_simplier(s string) string {
	return strings.ToLower(strings.Replace(s, "_", "", -1))
}

func get_permission_level(user *discordgo.User, server string) int {
	if user.ID == discord_superuser_id || user.ID == discord_bot_user_id {
		return PERMISSIONS_SUPERUSER //bot admin
	}
	ckey := local_users[user.ID]
	if ckey == "" {
		return PERMISSIONS_NONE //not registered
	}

	ckey = ckey_simplier(ckey)
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
		if check_bans(message.Author, BANTYPE_COMMANDS, false) != "" && command != "baninfo" {
			reply(session, message, "you're banned from this action. Try !baninfo", DEL_DEFAULT)
			return
		}
		if len(args) > 1 {
			args = args[1:]
		} else {
			args = make([]string, 0) //empty slice
		}
		log.Println(message.Author.String() + " c-> " + message.ContentWithMentionsReplaced())
		dcomm, ok := Known_commands[command]
		if !ok {
			reply(session, message, "unknown command: `"+Dweaksanitize(command)+"`", DEL_DEFAULT)
			return
		}
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
	}

	if known_channels_id_t[message.ChannelID].generic_type != "ooc" && known_channels_id_t[message.ChannelID].generic_type != "admin" {
		return
	}
	if !spam_check(message.Author.ID) {
		delete_in(session, message.Message, 1)
		return
	}
	shown_nick := local_users[message.Author.ID]
	if shown_nick == "" {
		defer delcommand(session, message)
		channel, err := session.Channel(message.ChannelID)
		if err != nil {
			log.Println("Shiet: ", err)
			reply(session, message, "failed to retrieve channel", DEL_DEFAULT)
		}
		if logoff_user(channel.GuildID, message.Author.ID) {
			reply(session, message, "you were logged off because of missing registration entry (try !register)", DEL_NEVER)
			return
		}
	}
	addstr := ""
	srv := known_channels_id_t[message.ChannelID]
	mcontent = emoji_stripper.ReplaceAllString(mcontent, "")
	var byondmcontent string //sent to byond
	if !Permissions_check(message.Author, PERMISSIONS_ADMIN, srv.server) {
		byondmcontent = strings.Replace(mcontent, "\n", "#", -1)
		byondmcontent = html.EscapeString(byondmcontent)
	} else {
		byondmcontent = "<font color='#39034f'>" + mcontent + "</font>"
		addstr = "&isadmin=1"
	}
	switch srv.generic_type {
	case "ooc":
		if check_bans(message.Author, BANTYPE_OOC, false) != "" {
			defer delcommand(session, message)
			reply(session, message, "you're banned from this action. Try !baninfo", DEL_DEFAULT)
			return
		}
		br := Byond_query(srv.server, "ckey="+Bquery_convert(shown_nick)+"&ooc="+Bquery_convert(byondmcontent)+addstr, true)
		if br.String() == "muted" {
			defer delcommand(session, message)
			reply(session, message, "your ckey is muted from OOC", DEL_DEFAULT)
			return
		}
		if br.String() == "globally muted" {
			defer delcommand(session, message)
			reply(session, message, "OOC is globally muted", DEL_DEFAULT)
			return
		}
		Discord_message_propagate(srv.server, "ooc", "DISCORD OOC:", shown_nick, strip.StripTags(mcontent), message.ChannelID)
	case "admin":
		Byond_query(srv.server, "admin="+Bquery_convert(shown_nick)+"&asay="+Bquery_convert(byondmcontent), true)
		Discord_message_propagate(srv.server, "admin", "DISCORD ASAY:", shown_nick, strip.StripTags(mcontent), message.ChannelID)
	default:
	}
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
		_, err := dsession.ChannelMessageSend(id, rid+subs+Dsanitize(message))
		if err != nil {
			log.Println("DISCORD ERROR: failed to send message to discord: ", err)
		}
	}
}

func Discord_message_send(servername, channel, prefix, ckey, message string) {
	defer logging_recover("Dms")
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
		_, err := dsession.ChannelMessageSend(id, "**"+Dsanitize(prefix+delim+ckey)+":** "+Dsanitize(message))
		if err != nil {
			log.Println("DISCORD ERROR: failed to send message to discord: ", err)
		}
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
		_, err := dsession.ChannelMessageSend(id, message)
		if err != nil {
			log.Println("DISCORD ERROR: failed to send message to discord: ", err)
		}
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
			log.Println("DISCORD ERROR: failed to send embed to discord: ", err)
		}
	}
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
		_, err := dsession.ChannelMessageSend(id, "**"+Dsanitize(prefix+delim+ckey)+":** "+Dsanitize(message))
		if err != nil {
			log.Println("DISCORD ERROR: failed to send message to discord: ", err)
		}
	}
}

func Discord_private_message_send(user *discordgo.User, message string) bool {
	channel, err := dsession.UserChannelCreate(user.ID)
	if err != nil {
		log.Println("Failed to create private channel: ", err)
		return false
	}
	_, err = dsession.ChannelMessageSend(channel.ID, message)
	if err != nil {
		log.Println("DISCORD ERROR: failed to send message to discord: ", err)
	}
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
	rows, err := Database.Query("select CHANTYPE, CHANID, SRVNAME from DISCORD_CHANNELS")
	if err != nil {
		panic(err)
	}
	for k := range known_channels_id_t {
		delete(known_channels_id_t, k)
	} //clear id->type pairs
	for k, v := range known_channels_s_t_id_m {
		for sk := range v {
			delete(v, sk)
		}
		delete(known_channels_s_t_id_m, k)
	} //clear server->type->ids and server->type
	for rows.Next() {
		var ch, id, srv string
		if terr := rows.Scan(&ch, &id, &srv); terr != nil {
			log.Println("DB ERROR: ", terr)
			continue
		}
		ch = trim(ch)
		id = trim(id)
		srv = trim(srv)
		known_channels_id_t[id] = channel{ch, srv}
		if srv == "" {
			log.Println("DB: setting `" + id + "` to '" + srv + "@" + ch + "';")
			continue
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
}

func add_known_channel(srv, t, id, gid string) bool {
	result, err := Database.Exec("insert into DISCORD_CHANNELS values ($1, $2, $3, $4);", t, id, gid, srv)
	if err != nil {
		log.Println("DB ERROR: failed to insert: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_channels() //update everything
		return true
	}
	return false
}

func Remove_known_channels(srv, t, gid string) bool {
	var result sql.Result
	var err error
	if gid == "" {
		result, err = Database.Exec("delete from DISCORD_CHANNELS where CHANTYPE = $1 and SRVNAME = $2;", t, srv)
	} else {
		result, err = Database.Exec("delete from DISCORD_CHANNELS where CHANTYPE = $1 and GUILDID = $2 and SRVNAME = $3;", t, gid, srv)
	}
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_channels() //update everything
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
	result, err := Database.Exec("update DISCORD_CHANNELS set CHANID = $2 where CHANTYPE = $1 and GUILDID = $3 and SRVNAME = $4;", t, id, gid, srv)
	if err != nil {
		log.Println("DB ERROR: failed to update: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_channels() //update everything
		return true
	} else {
		return add_known_channel(srv, t, id, gid)
	}
}

func Remove_token(ttype, data string) bool {
	result, err := Database.Exec("delete from DISCORD_TOKENS where TYPE = $1 and DATA = $2;", ttype, data)
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		return true
	}
	return false
}

func remove_token_by_id(id string) bool {
	result, err := Database.Exec("delete from DISCORD_TOKENS where TOKEN = $1;", id)
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		return true
	}
	return false
}

func Create_token(ttype, data string) string {
	id := uuid.New().String()
	result, err := Database.Exec("insert into DISCORD_TOKENS values ($1, $2, $3);", id, ttype, data)
	if err != nil {
		log.Println("DB ERROR: failed to insert: ", err)
		return ""
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return ""
	}
	if affected > 0 {
		return id
	}
	return ""
}

func expend_token(id string) (ttype, data string) {
	row := Database.QueryRow("select TYPE, DATA from DISCORD_TOKENS where TOKEN = $1", id)
	err := row.Scan(&ttype, &data)
	if err != nil {
		log.Println("DB ERROR: failed to retrieve token data: ", err)
		return
	}
	remove_token_by_id(id)
	ttype = trim(ttype)
	data = trim(data)
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

func register_user(login, ckey string) {
	defer logging_recover("ru:")
	_, err := Database.Exec("delete from DISCORD_REGISTERED_USERS where DISCORDID = $1;", login)
	if err != nil {
		panic(err)
	}
	_, err = Database.Exec("delete from DISCORD_REGISTERED_USERS where CKEY = $1;", ckey)
	if err != nil {
		panic(err)
	}
	_, err = Database.Exec("insert into DISCORD_REGISTERED_USERS values ($1, $2);", login, ckey)
	if err != nil {
		panic(err)
	}
	update_local_user(login)
	user, err := dsession.User(login)
	if err != nil {
		panic("failed to get user")
	}
	Discord_private_message_send(user, "Registered as `"+ckey+"`")
}

func update_local_users() {
	rows, err := Database.Query("select DISCORDID, CKEY from DISCORD_REGISTERED_USERS")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve known channels: ", err)
		return
	}
	for k := range local_users {
		delete(local_users, k)
	}
	for rows.Next() {
		var login, ckey string
		if terr := rows.Scan(&login, &ckey); terr != nil {
			log.Println("DB ERROR: ", terr)
			continue
		}
		login = trim(login)
		ckey = trim(ckey)
		local_users[login] = ckey
	}
}

func update_local_user(login string) (ckey string) {
	row := Database.QueryRow("select CKEY from DISCORD_REGISTERED_USERS where DISCORDID = $1", login)
	err := row.Scan(&ckey)
	if err != nil {
		log.Println("DB ERROR: failed to retrieve token data: ", err)
		return
	}
	ckey = trim(ckey)
	for l, c := range local_users {
		if l == login || c == ckey {
			delete(local_users, l)
		}
	}
	local_users[login] = ckey
	return
}

func populate_known_roles() {
	rows, err := Database.Query("select GUILDID, ROLEID, ROLETYPE, SRVNAME from DISCORD_ROLES")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve known roles: ", err)
		return
	}
	//clean known
	for k := range discord_player_roles {
		delete(discord_player_roles, k)
	}
	for k := range discord_admin_roles {
		delete(discord_admin_roles, k)
	}
	for rows.Next() {
		var gid, rid, tp, srv string
		if terr := rows.Scan(&gid, &rid, &tp, &srv); terr != nil {
			log.Println("DB ERROR: ", terr)
			continue
		}
		gid = trim(gid)
		rid = trim(rid)
		tp = trim(tp)
		srv = trim(srv)
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
}
func update_known_role(gid, tp, rid, srv string) bool {
	result, err := Database.Exec("update DISCORD_ROLES set ROLEID = $1 where GUILDID = $2 and ROLETYPE = $3 and SRVNAME = $4;", rid, gid, tp, srv)
	if err != nil {
		log.Println("DB ERROR: failed to update: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_roles()
		return true
	}
	return create_known_role(gid, tp, rid, srv)
}
func create_known_role(gid, tp, rid, srv string) bool {
	result, err := Database.Exec("insert into DISCORD_ROLES values($1, $2, $3, $4);", gid, rid, tp, srv)
	if err != nil {
		log.Println("DB ERROR: failed to insert: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_roles()
		return true
	}
	return false
}
func remove_known_role(gid, tp, srv string) bool {
	result, err := Database.Exec("delete from DISCORD_ROLES where GUILDID = $1 and ROLETYPE = $2 and SRVNAME = $3;", gid, tp, srv)
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_known_roles()
		return true
	}
	return false
}

func populate_bans() {
	rows, err := Database.Query("select CKEY, REASON, ADMIN, TYPE, PERMISSION from DISCORD_BANS")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve known bans: ", err)
		return
	}
	//clean known
	for k := range known_bans {
		delete(known_bans, k)
	}
	for rows.Next() {
		var ckey, reason, admin string
		var bantype, permission int
		if terr := rows.Scan(&ckey, &reason, &admin, &bantype, &permission); terr != nil {
			log.Println("DB ERROR: ", terr)
		}
		ckey = trim(ckey)
		reason = trim(reason)
		admin = trim(admin)
		known_bans[ckey] = dban{reason, admin, bantype, permission}
	}
}

func update_ban(ckey, reason string, user *discordgo.User, tp int) bool {
	ckey = strings.ToLower(ckey)
	permissions := get_permission_level(user, "")
	if permissions < PERMISSIONS_ADMIN {
		return false
	}
	admin := local_users[user.ID]
	result, err := Database.Exec("SELECT * from DISCORD_BANS where CKEY = $1 ;", ckey)
	if err != nil {
		log.Println("DB ERROR: failed to select: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		bn, ok := known_bans[ckey]
		if ok {
			tp |= bn.bantype
		}
		result, err = Database.Exec("update DISCORD_BANS set TYPE = $1::numeric where CKEY = $3 and PERMISSION <= $2::numeric ;", tp, permissions, ckey)
		if err != nil {
			log.Println("DB ERROR: failed to update: ", err)
			return false
		}
		affected, err := result.RowsAffected()
		if err != nil {
			log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
			return false
		}
		if affected > 0 {
			populate_bans()
			return true
		}
	} else {
		// no such entry, create new
		result, err = Database.Exec("insert into DISCORD_BANS values($1, $2, $3, $4, $5) ;", ckey, admin, reason, tp, permissions)
		if err != nil {
			log.Println("DB ERROR: failed to update: ", err)
			return false
		}
		affected, err = result.RowsAffected()
		if err != nil {
			log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
			return false
		}
		if affected > 0 {
			populate_bans()
			return true
		}
	}
	return false
}

func remove_ban(ckey string, user *discordgo.User) bool {
	ckey = strings.ToLower(ckey)
	permissions := get_permission_level(user, "")
	if permissions < PERMISSIONS_ADMIN {
		return false
	}
	result, err := Database.Exec("delete from DISCORD_BANS where CKEY = $1 and PERMISSION <= $2::numeric ;", ckey, permissions)
	if err != nil {
		log.Println("DB ERROR: failed to update: ", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return false
	}
	if affected > 0 {
		populate_bans()
		return true
	}
	return false
}

func check_bans(user *discordgo.User, tp int, forced bool) string {
	ckey := local_users[user.ID]
	ckey = strings.ToLower(ckey)
	if ckey == "" {
		return ""
	}
	ban, ok := known_bans[ckey]
	if !ok {
		return ""
	}

	if (ban.bantype & tp) == 0 {
		return "" //no matching ban
	}
	if Permissions_check(user, ban.permlevel, "") && !forced {
		return "" //avoid bans from same level
	}
	bantype := make([]string, 0)
	if ban.bantype&BANTYPE_OOC != 0 {
		bantype = append(bantype, BANSTRING_OOC)
	}
	if ban.bantype&BANTYPE_COMMANDS != 0 {
		bantype = append(bantype, BANSTRING_COMMANDS)
	}
	bantypestring := strings.Join(bantype, ", ")
	return "You were banned from " + bantypestring + " by " + ban.admin + " with following reason:\n" + ban.reason
}

func subscribe_user(guildid, userid, srv string) bool {
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
	ret := count_query("select * from DISCORD_ONETIME_SUBSCRIPTIONS where USERID = '" + userid + "' and GUILDID = '" + guildid + "' and SRVNAME = '" + srv + "';")
	if ret == -1 {
		return false
	}
	if ret == 0 {
		if count_query("insert into DISCORD_ONETIME_SUBSCRIPTIONS values('"+userid+"','"+guildid+"','"+srv+"');") < 1 {
			return false
		}
	}
	return true
}

func flush_onetime_subscriptions(servername string) {
	for k, v := range discord_onetime_subscriptions {
		for l := range v {
			delete(v, l)
		}
		delete(discord_onetime_subscriptions, k)
	} //delete in any case
	rows, err := Database.Query("select USERID, GUILDID, SRVNAME from DISCORD_ONETIME_SUBSCRIPTIONS;")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve subs: ", err)
		return
	}
	for rows.Next() {
		var userid, guildid, srv string
		if terr := rows.Scan(&userid, &guildid, &srv); terr != nil {
			log.Println("DB ERROR: ", terr)
			continue
		}
		userid = trim(userid)
		guildid = trim(guildid)
		srv = trim(srv)
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
	_, err = Database.Exec("delete from DISCORD_ONETIME_SUBSCRIPTIONS;")
	if err != nil {
		log.Println("ERROR: del: ", err)
	}
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
			update_ban(ckey, "SPAM SPAM SPAM", dsession.State.User, BANTYPE_OOC|BANTYPE_COMMANDS)
		}
		discord_spam_prot_bans[userid] = true
	}
	if discord_spam_prot_bans[userid] {
		return ccnt == 0
	}
	return true
}

func launch_spam_ticker() chan int {
	quit := make(chan int)
	go spam_ticker(quit)
	return quit
}

func stop_spam_ticker(quit chan int) {
	quit <- 0
}

func spam_ticker(quit chan int) {
	tick := time.Tick(time.Duration(discord_spam_prot_tick) * time.Second)
	for {
		select {
		case <-quit:
			return
		case <-tick:
			for uid, _ := range discord_spam_prot_checks {
				discord_spam_prot_checks[uid] = 0
			}
		}
	}
}

var spamticker chan int

func Dopen() {
	var err error
	dsession.State.User, err = dsession.User("@me")
	if err != nil {
		log.Fatalln("User fetch error: ", err)
	}
	discord_bot_user_id = dsession.State.User.ID
	err = dsession.Open()
	if err != nil {
		log.Fatalln("Session Open error: ", err)
	}
	log.Print("Successfully connected to discord, now running as ", dsession.State.User)
	populate_known_channels()
	update_local_users()
	populate_known_roles()
	populate_bans()
	Load_admins()
	spamticker = launch_spam_ticker()
	dsession.AddHandler(messageCreate)
	for _, srv := range known_servers {
		Discord_message_send(srv.name, "bot_status", "BOT", "STATUS UPDATE", "now running.")
	}
}

func Dclose() {
	for _, srv := range known_servers {
		Discord_message_send(srv.name, "bot_status", "BOT", "STATUS UPDATE", "shutting down due to host request.")
	}
	stop_spam_ticker(spamticker)
	err := dsession.Close()
	if err != nil {
		log.Fatal("Failed to close dsession: ", err)
	}
}
