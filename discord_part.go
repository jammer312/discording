package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	discord_bot_token   string
	discord_ooc_channel string
)

var DSession, _ = discordgo.New()

func init() {
	discord_bot_token = os.Getenv("discord_bot_token")
	if discord_bot_token == "" {
		log.Fatalln("Failed to retrieve $discord_bot_token")
	}
	DSession.Token = discord_bot_token
	discord_ooc_channel = os.Getenv("discord_ooc_channel")
	if discord_ooc_channel == "" {
		log.Fatalln("Failed to retrieve $discord_ooc_channel")
	}
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == DSession.State.User.ID {
		return
	}
	/*	_, err := session.ChannelMessageSend(message.ChannelID, "Message sent by <@"+message.Author.ID+"> in this channel with contents `"+message.Content+"`")
		if err != nil {
			log.Fatal("Message send error: ", err)
		}*/
}

func usage_example() {
	var err error
	DSession.State.User, err = DSession.User("@me")
	if err != nil {
		log.Fatalln("User fetch error: ", err)
	}
	err = DSession.Open()
	if err != nil {
		log.Fatalln("Session Open error: ", err)
	}
	log.Print("Successfully connected to discord, now running as ", DSession.State.User)
	DSession.AddHandler(messageCreate)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	defer DSession.Close()
}
