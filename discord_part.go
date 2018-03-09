package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/grokify/html-strip-tags-go"
	"html"
	"log"
	"os"
	"strings"
)

var (
	discord_bot_token         string
	Discord_command_character string
	known_channels_id_t       map[string]string
	known_channels_t_id       map[string]string
	local_users               map[string]string
	Known_admins              []string
	discord_superuser_id      string

	discord_player_roles     map[string]string   //guildid -> role
	discord_subscriber_roles map[string]string   //guilldid -> role
	discord_admin_roles      map[string]string   //guildid -> role
	known_channels_t_id_m    map[string][]string //type -> arr of ids
)

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

type dban struct {
	reason    string
	admin     string
	bantype   int
	permlevel int
}

var known_bans map[string]dban

var dsession, _ = discordgo.New()

func init() {
	discord_bot_token = os.Getenv("discord_bot_token")
	if discord_bot_token == "" {
		log.Fatalln("Failed to retrieve $discord_bot_token")
	}
	dsession.Token = discord_bot_token

	Discord_command_character = os.Getenv("discord_command_character")
	if Discord_command_character == "" {
		log.Fatalln("Failed to retrieve $discord_command_character")
	}
	discord_superuser_id = os.Getenv("discord_superuser_id")
	if discord_superuser_id == "" {
		log.Fatalln("Failed to retrieve $discord_superuser_id")
	}

	known_channels_id_t = make(map[string]string)
	known_channels_t_id = make(map[string]string)
	known_bans = make(map[string]dban)
	local_users = make(map[string]string)
	Known_admins = make([]string, 0)

	discord_player_roles = make(map[string]string)
	discord_admin_roles = make(map[string]string)
	discord_subscriber_roles = make(map[string]string)
	known_channels_t_id_m = make(map[string][]string)

}

func reply(session *discordgo.Session, message *discordgo.MessageCreate, msg string) {
	_, err := session.ChannelMessageSend(message.ChannelID, "<@!"+message.Author.ID+">, "+msg)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to send reply message to discord: ", err)
	}
}

func delcommand(session *discordgo.Session, message *discordgo.MessageCreate) {
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

func get_permission_level(user *discordgo.User) int {
	if user.ID == discord_superuser_id {
		return PERMISSIONS_SUPERUSER //bot admin
	}
	ckey := local_users[user.ID]
	if ckey == "" {
		return PERMISSIONS_NONE //not registered
	}

	ckey = strings.ToLower(ckey)

	for _, admin := range Known_admins {
		if ckey == strings.ToLower(admin) {
			return PERMISSIONS_ADMIN //generic admin
		}
	}
	return PERMISSIONS_REGISTERED
}

func Permissions_check(user *discordgo.User, permission_level int) bool {
	return get_permission_level(user) >= permission_level
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID {
		return
	}
	mcontent := message.ContentWithMentionsReplaced()
	if len(mcontent) < 2 { //one for command char and at least one for command
		return
	}
	if mcontent[:1] == Discord_command_character {
		//it's command
		defer delcommand(session, message)
		args := strings.Fields(mcontent[1:])
		command := strings.ToLower(args[0])
		if check_bans(message.Author, BANTYPE_COMMANDS, false) != "" && command != "baninfo" {
			reply(session, message, "you're banned from this action. Try !baninfo")
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
			reply(session, message, "unknown command: `"+Dweaksanitize(command)+"`")
			return
		}
		if !Permissions_check(message.Author, dcomm.Permlevel) {
			reply(session, message, "missing permissions required to run this command: `"+Dweaksanitize(command)+"`")
			return
		}
		if len(args) < dcomm.Minargs {
			reply(session, message, "usage: "+dcomm.Usagestr())
			return
		}
		ret := dcomm.Exec(session, message, args)
		if ret == "" {
			return
		}
		reply(session, message, ret)
		return
	}

	if known_channels_id_t[message.ChannelID] != "ooc" && known_channels_id_t[message.ChannelID] != "admin" {
		return
	}

	shown_nick := local_users[message.Author.ID]
	if shown_nick == "" {
		defer delcommand(session, message)
		channel, err := session.Channel(message.ChannelID)
		if err != nil {
			log.Println("Shiet: ", err)
			reply(session, message, "failed to retrieve channel")
		}
		if logoff_user(channel.GuildID, message.Author.ID) {
			reply(session, message, "you were logged off because of missing registration entry (try !register)")
			return
		}
	}
	addstr := ""
	if !Permissions_check(message.Author, PERMISSIONS_ADMIN) {
		mcontent = html.EscapeString(mcontent)
	} else {
		mcontent = "<font color='#39034f'>" + mcontent + "</font>"
		addstr = "&isadmin=1"
	}

	switch known_channels_id_t[message.ChannelID] {
	case "ooc":
		if check_bans(message.Author, BANTYPE_OOC, false) != "" {
			defer delcommand(session, message)
			reply(session, message, "you're banned from this action. Try !baninfo")
			return
		}
		br := Byond_query("admin="+Bquery_convert(shown_nick)+"&ooc="+Bquery_convert(mcontent)+addstr, true)
		if br.String() == "muted" {
			defer delcommand(session, message)
			reply(session, message, "your ckey is muted from OOC")
			return
		}
		if br.String() == "globally muted" {
			defer delcommand(session, message)
			reply(session, message, "OOC is globally muted")
			return
		}
		Discord_message_propagate("ooc", "DISCORD OOC:", shown_nick, strip.StripTags(mcontent), message.ChannelID)
	case "admin":
		Byond_query("admin="+Bquery_convert(shown_nick)+"&asay="+Bquery_convert(mcontent), true)
		Discord_message_propagate("admin", "DISCORD ASAY:", shown_nick, strip.StripTags(mcontent), message.ChannelID)
	default:
	}
}

func Discord_subsriber_message_send(channel, message string) {
	channels, ok := known_channels_t_id_m[channel]
	if !ok || len(channels) < 1 {
		return //no bound channels
	}
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
		rid, ok := discord_subscriber_roles[guild.ID]
		if !ok {
			continue
		}
		_, err := dsession.ChannelMessageSend(id, "<@!"+rid+">, "+Dsanitize(message))
		if err != nil {
			log.Println("DISCORD ERROR: failed to send message to discord: ", err)
		}
	}
}

func Discord_message_send(channel, prefix, ckey, message string) {
	channels, ok := known_channels_t_id_m[channel]
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

func Discord_message_propagate(channel, prefix, ckey, message, chanid string) {
	//given channel id and other params sends message to all channels except specified one
	channels, ok := known_channels_t_id_m[channel]
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
	rows, err := Database.Query("select CHANTYPE, CHANID from DISCORD_CHANNELS")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve known channels: ", err)
		return
	}
	for k := range known_channels_id_t {
		delete(known_channels_id_t, k)
	} //clear id->type pairs
	for k := range known_channels_t_id_m {
		delete(known_channels_t_id_m, k)
	} //clear type->ids
	for rows.Next() {
		var ch, id string
		if terr := rows.Scan(&ch, &id); terr != nil {
			log.Println("DB ERROR: ", terr)
		}
		ch = strings.Trim(ch, " ")
		id = strings.Trim(id, " ")
		known_channels_id_t[id] = ch
		chsl, ok := known_channels_t_id_m[ch]
		if !ok {
			chsl = make([]string, 0)
		}
		known_channels_t_id_m[ch] = append(chsl, id)
		log.Println("DB: setting `" + id + "` to '" + ch + "';")
	}
}

func add_known_channel(t, id, gid string) bool {
	result, err := Database.Exec("insert into DISCORD_CHANNELS values ($1, $2, $3);", t, id, gid)
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

func Remove_known_channels(t, gid string) bool {
	var result sql.Result
	var err error
	if gid == "" {
		result, err = Database.Exec("delete from DISCORD_CHANNELS where CHANTYPE = $1 ;", t)
	} else {
		result, err = Database.Exec("delete from DISCORD_CHANNELS where CHANTYPE = $1 and GUILDID = $2", t, gid)
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
		ret += fmt.Sprintf("`%s` <-> <#%s>\n", t, id)
	}
	return ret
}

func Update_known_channel(t, id, gid string) bool {
	result, err := Database.Exec("update DISCORD_CHANNELS set CHANID = $2 where CHANTYPE = $1 and GUILDID = $3;", t, id, gid)
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
		return add_known_channel(t, id, gid)
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
	_, err := Database.Exec("delete from DISCORD_REGISTERED_USERS where DISCORDID = $1;", login)
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return
	}
	_, err = Database.Exec("delete from DISCORD_REGISTERED_USERS where CKEY = $1;", ckey)
	if err != nil {
		log.Println("DB ERROR: failed to delete: ", err)
		return
	}
	_, err = Database.Exec("insert into DISCORD_REGISTERED_USERS values ($1, $2);", login, ckey)
	if err != nil {
		log.Println("DB ERROR: failed to insert: ", err)
		return
	}
	update_local_user(login)
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
	rows, err := Database.Query("select GUILDID, ROLEID, ROLETYPE from DISCORD_ROLES")
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
		var gid, rid, tp string
		if terr := rows.Scan(&gid, &rid, &tp); terr != nil {
			log.Println("DB ERROR: ", terr)
		}
		gid = trim(gid)
		rid = trim(rid)
		tp = trim(tp)
		switch tp {
		case ROLE_PLAYER:
			discord_player_roles[gid] = rid
		case ROLE_ADMIN:
			discord_admin_roles[gid] = rid
		case ROLE_SUBSCRIBER:
			discord_subscriber_roles[gid] = rid
		}
	}
}
func update_known_role(gid, tp, rid string) bool {
	result, err := Database.Exec("update DISCORD_ROLES set ROLEID = $1 where GUILDID = $2 and ROLETYPE = $3 ;", rid, gid, tp)
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
	return create_known_role(gid, tp, rid)
}
func create_known_role(gid, tp, rid string) bool {
	result, err := Database.Exec("insert into DISCORD_ROLES values($1, $2, $3);", gid, rid, tp)
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
func remove_known_role(gid, tp string) bool {
	result, err := Database.Exec("delete from DISCORD_ROLES where GUILDID = $1 and ROLETYPE = $2;", gid, tp)
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
	permissions := get_permission_level(user)
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
	permissions := get_permission_level(user)
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
	if Permissions_check(user, ban.permlevel) && !forced {
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

func subscribe_user(guildid, userid string) bool {
	ckey := update_local_user(userid)
	if ckey == "" {
		return false
	}
	ckey = strings.ToLower(ckey)
	var subscriber_role string
	var ok bool
	subscriber_role, ok = discord_subscriber_roles[guildid]
	if !ok {
		log.Println("Failed to find subscriber role")
		return false
	}
	err := dsession.GuildMemberRoleAdd(guildid, userid, subscriber_role)
	if err != nil {
		log.Println("Subscribe error: ", err)
		return false
	}
	return true
}
func unsubscribe_user(guildid, userid string) bool {
	ckey := update_local_user(userid)
	if ckey == "" {
		return false
	}
	ckey = strings.ToLower(ckey)
	var subscriber_role string
	var ok bool
	subscriber_role, ok = discord_subscriber_roles[guildid]
	if !ok {
		log.Println("Failed to find subscriber role")
		return false
	}
	err := dsession.GuildMemberRoleRemove(guildid, userid, subscriber_role)
	if err != nil {
		log.Println("Subscribe error: ", err)
		return false
	}
	return true
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
		log.Println("Failed to find player role")
		return false
	}
	err := dsession.GuildMemberRoleAdd(guildid, userid, player_role)
	if err != nil {
		log.Println("Login error: ", err)
		return false
	}

	isadmin := false
	for _, admin := range Known_admins {
		if ckey == admin {
			isadmin = true
			break
		}
	}

	if !isadmin {
		return true
	}
	var admin_role string
	admin_role, ok = discord_admin_roles[guildid]
	if !ok {
		log.Println("Failed to find admin role")
		return false
	}
	err = dsession.GuildMemberRoleAdd(guildid, userid, admin_role)
	if err != nil {
		log.Println("Login error: ", err)
		return false
	}

	return true
}

func logoff_user(guildid, userid string) bool {
	var player_role string
	var ok bool
	player_role, ok = discord_player_roles[guildid]
	if !ok {
		log.Println("Failed to find player role")
		return false
	}
	err := dsession.GuildMemberRoleRemove(guildid, userid, player_role)
	if err != nil {
		log.Println("Logoff error: ", err)
		return false
	}
	var admin_role string
	admin_role, ok = discord_admin_roles[guildid]
	if !ok {
		log.Println("Failed to find admin role")
		return false
	}
	err = dsession.GuildMemberRoleRemove(guildid, userid, admin_role)
	if err != nil {
		log.Println("Logoff error: ", err)
		return false
	}
	return true

}

func Dopen() {
	var err error
	dsession.State.User, err = dsession.User("@me")
	if err != nil {
		log.Fatalln("User fetch error: ", err)
	}
	err = dsession.Open()
	if err != nil {
		log.Fatalln("Session Open error: ", err)
	}
	log.Print("Successfully connected to discord, now running as ", dsession.State.User)
	populate_known_channels()
	update_local_users()
	populate_known_roles()
	populate_bans()
	Load_admins(&Known_admins)
	dsession.AddHandler(messageCreate)
	Discord_message_send("bot_status", "BOT", "STATUS UPDATE", "now running.")
}

func Dclose() {
	Discord_message_send("bot_status", "BOT", "STATUS UPDATE", "shutting down due to host request.")
	err := dsession.Close()
	if err != nil {
		log.Fatal("Failed to close dsession: ", err)
	}
}
