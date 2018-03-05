package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"strings"
)

var (
	discord_bot_token   string
	discord_ooc_channel string
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
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == dsession.State.User.ID {
		return
	}
	/*	_, err := session.ChannelMessageSend(message.ChannelID, "Message sent by <@"+message.Author.ID+"> in this channel with contents `"+message.Content+"`")
		if err != nil {
			log.Fatal("Message send error: ", err)
		}*/
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
	return out
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
	dsession.AddHandler(messageCreate)
}

func Dclose() {
	err := dsession.Close()
	if err != nil {
		log.Fatal("Failed to close dsession: ", err)
	}
}
