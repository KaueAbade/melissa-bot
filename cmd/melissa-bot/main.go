package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// Variables used for configuration
var (
	discord      *discordgo.Session
	discordToken string
	commandWipe  bool
)

// Get bot configurations
func init() {
	// Get the bot token from an environment variable
	discordToken = env.GetStr("DISCORD_BOT_TOKEN", "")
	if discordToken == "" {
		log.Fatal("No token provided\nIt is necessary to set the DISCORD_BOT_TOKEN environment variable")
	}

	// Check if the command wipe flag is set
	commandWipe = env.GetBool("COMMAND_WIPE", false)
}

// Setup the Discord session and event handlers
func init() {
	var err error

	// Create a new Discord session using the provided bot token
	log.Println("Starting Melissa Bot...")
	discord, err = discordgo.New("Bot " + discordToken)
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
	signal.Notify(quitSignal, syscall.SIGTERM)
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
