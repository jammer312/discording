package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"strings"
)

var (
	discord_bot_token         string
	discord_ooc_channel       string
	discord_command_character string
	known_channels            map[string]string
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
	known_channels = make(map[string]string)
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

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID {
		return
	}
	mcontent := message.ContentWithMentionsReplaced()
	if len(mcontent) < 1 {
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
		default:
			reply(session, message, "unknown command: `"+Dsanitize(command)+"`")
			delcommand(session, message)
		}
		return

	}
	if message.ChannelID == discord_ooc_channel {
		Byond_query("admin="+Bquery_convert(message.Author.Username)+"&ooc="+Bquery_convert(mcontent), true)
	}
}

func OOC_message_send(m string) {
	_, err := dsession.ChannelMessageSend(discord_ooc_channel, "**OOC:** "+m)
	if err != nil {
		log.Println("NON-PANIC ERROR: failed to send OOC message to discord: ", err)
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
