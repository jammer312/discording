package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"strings"
)

var (
	discord_bot_token         string
	discord_ooc_channel       string
	discord_command_character string
	known_channels_id_t       map[string]string
	known_channels_t_id       map[string]string
	discord_superuser_id      string
)

var dsession, _ = discordgo.New()

func init() {
	discord_bot_token = os.Getenv("discord_bot_token")
	if discord_bot_token == "" {
		log.Fatalln("Failed to retrieve $discord_bot_token")
	}
	dsession.Token = discord_bot_token
	discord_ooc_channel = os.Getenv("discord_ooc_channel")
	if discord_ooc_channel == "" {
		log.Fatalln("Failed to retrieve $discord_ooc_channel")
	}
	discord_command_character = os.Getenv("discord_command_character")
	if discord_command_character == "" {
		log.Fatalln("Failed to retrieve $discord_command_character")
	}
	discord_superuser_id = os.Getenv("discord_superuser_id")
	if discord_superuser_id == "" {
		log.Fatalln("Failed to retrieve $discord_superuser_id")
	}
	known_channels_id_t = make(map[string]string)
	known_channels_t_id = make(map[string]string)
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

func permissions_check(user *discordgo.User) bool {
	return user.ID == discord_superuser_id //maybe it's only placeholder
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID {
		return
	}
	mcontent := message.ContentWithMentionsReplaced()
	if len(mcontent) < 2 { //one for command char and at least one for command
		return
	}
	if mcontent[:1] == discord_command_character {
		//it's command
		args := strings.Split(mcontent[1:], " ")
		command := args[0]
		if len(args) > 1 {
			args = args[1:]
		} else {
			args = make([]string, 0) //empty slice
		}
		switch command {
		case "ping":
			reply(session, message, "pong!")
			delcommand(session, message)
			return
		case "count":
			reply(session, message, fmt.Sprint(len(args))+" args detected")
			delcommand(session, message)
			return
		case "here":
			delcommand(session, message)
			if len(args) < 1 {
				reply(session, message, "usage: !here [channel_type]")
				return
			}
			if !permissions_check(message.Author) {
				reply(session, message, "permission check failed")
				return
			}
			if update_known_channels(args[0], message.ChannelID) {
				reply(session, message, "changed `"+Dsanitize(args[0])+"` channel type to <#"+message.ChannelID+">")
			} else {
				reply(session, message, "failed to change `"+Dsanitize(args[0])+"` channel type to <#"+message.ChannelID+">")
			}
			return
		case "unbind":
			delcommand(session, message)
			if len(args) < 1 {
				reply(session, message, "usage: !unbind [channel_type]")
				return
			}
			if !permissions_check(message.Author) {
				reply(session, message, "permission check failed")
				return
			}
			if update_known_channels(args[0], "") {
				reply(session, message, "unbound `"+Dsanitize(args[0])+"`"
			} else {
				reply(session, message, "failed to unbind `"+Dsanitize(args[0])+"`"
			}
			return
		default:
			reply(session, message, "unknown command: `"+Dsanitize(command)+"`")
			delcommand(session, message)
		}
		return

	}
	switch known_channels_id_t[message.ChannelID] {
	case "ooc":
		Byond_query("admin="+Bquery_convert(message.Author.Username)+"&ooc="+Bquery_convert(mcontent), true)
	case "admin":
		Byond_query("admin="+Bquery_convert(message.Author.Username)+"&asay="+Bquery_convert(mcontent), true)
	default:
	}
}

func Discord_message_send(channel, prefix, ckey, message string) {
	if known_channels_t_id[channel] == "" {
		return //idk where to send it
	}
	_, err := dsession.ChannelMessageSend(known_channels_t_id[channel], "**"+Dsanitize(prefix+" "+ckey)+"** "+Dsanitize(message))
	if err != nil {
		log.Println("DISCORD ERROR: failed to send OOC message to discord: ", err)
	}
}

func Dsanitize(m string) string {
	out := strings.Replace(m, "\\", "\\\\", -1)
	out = strings.Replace(out, "*", "\\*", -1)
	out = strings.Replace(out, "`", "\\`", -1)
	out = strings.Replace(out, "_", "\\_", -1)
	out = strings.Replace(out, "~", "\\~", -1)
	out = strings.Replace(out, "@", "\\@", -1)
	return out
}

func populate_known_channels() {
	rows, err := Database.Query("select CHANTYPE, CHANID from DISCORD_CHANNELS")
	if err != nil {
		log.Println("DB ERROR: failed to retrieve known channels: ", err)
		return
	}
	for rows.Next() {
		var ch, id string
		if terr := rows.Scan(&ch, &id); terr != nil {
			log.Println("DB ERROR: ", terr)
		}
		ch = strings.Trim(ch, " ")
		id = strings.Trim(id, " ")
		known_channels_id_t[id] = ch
		known_channels_t_id[ch] = id
		log.Println("DB: setting `" + id + "` to '" + ch + "';")
	}
}

func update_known_channels(t, id string) bool {
	result, err := Database.Exec("update DISCORD_CHANNELS set CHANID = $2 where CHANTYPE = $1;", t, id)
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
	}
	return false
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
	dsession.AddHandler(messageCreate)
}

func Dclose() {
	err := dsession.Close()
	if err != nil {
		log.Fatal("Failed to close dsession: ", err)
	}
}
