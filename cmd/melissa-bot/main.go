package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

// Variables used for configuration
var (
	Token string
)

func init() {
	// Get the bot token from an environment variable
	Token = os.Getenv("DISCORD_BOT_TOKEN")
	if Token == "" {
		log.Fatal("It is necessary to set the DISCORD_BOT_TOKEN environment variable")
	}
}

func main() {
	// Create a new Discord session using the provided bot token
	discord, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatal(err)
	}

	// Register the messageCreate func as a callback for MessageCreate events
	discord.AddHandler(messageCreate)

	// Specify the intents for the bot
	// For now it is simply required to listen for messages in order to reply to them
	discord.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Open a websocket connection to Discord and begin listening
	err = discord.Open()
	if err != nil {
		log.Fatal(err)
	}

	// Log!
	log.Println("Melissa Bot is now running.")

	// Wait here until CTRL-C or other sigterm is received
	quitSignal := make(chan os.Signal, 1)
	signal.Notify(quitSignal, os.Interrupt)
	<-quitSignal

	// Cleanly close down the Discord session
	err = discord.Close()
	if err != nil {
		log.Fatal(err)
	}
}

// This function will be called (due to AddHandler above)
// every time a new message is created on any channel that the
// bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// If the message is "ping" reply with "Pong!"
	if m.Content == "ping" {
		_, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
		if err != nil {
			log.Println("Error sending message:", err)
		}
	}

	// If the message is "pong" reply with "Ping!"
	if m.Content == "pong" {
		_, err := s.ChannelMessageSend(m.ChannelID, "Ping!")
		if err != nil {
			log.Println("Error sending message:", err)
		}
	}
}
