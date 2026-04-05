package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/KaueAbade/melissa-bot/internal/commands"
	"github.com/KaueAbade/melissa-bot/internal/env"
	"github.com/bwmarrin/discordgo"
)

// Variables used for configuration
var (
	discord      *discordgo.Session
	discordToken string

	commandWipe bool
	debug       bool
)

// Get bot configurations
func init() {
	// Get the bot token from an environment variable
	discordToken = env.GetStr("DISCORD_BOT_TOKEN", "")
	if discordToken == "" {
		log.Fatal("No token provided\nIt is necessary to set the DISCORD_BOT_TOKEN environment variable")
	}

	// Get configuration envs
	commandWipe = env.GetBool("WIPE_COMMANDS_ON_EXIT", false)
	debug = env.GetBool("DEBUG", false)
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

	// Specify the intents for the bot
	// For now it is simply required to listen for messages in order to reply to them
	log.Println("Setting the bots intents as GuildMessages, DirectMessages and MessageContent")
	discord.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Register the desired functions as callbacks for events
	log.Println("Setting event handlers for ready, message creation and interactions")
	discord.AddHandler(ready)
	discord.AddHandler(messageCreate)
	discord.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if handler, ok := commands.Handlers[interaction.ApplicationCommandData().Name]; ok {
			handler(session, interaction)
		}
	})
}

func main() {
	// Open a websocket connection to Discord and begin listening
	log.Println("Creating a websocket connection with the provided token")
	err := discord.Open()
	if err != nil {
		log.Fatal(err)
	}

	// Register the commands and their handlers
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands.Definitions))
	for i, cmd := range commands.Definitions {
		ccmd, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", cmd)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v\n", cmd.Name, err)
		}
		if debug {
			log.Printf("Registered command '%v'\n", cmd.Name)
		}
		registeredCommands[i] = ccmd
	}

	// Wait here until an sigterm is received
	quitSignal := make(chan os.Signal, 1)
	signal.Notify(quitSignal, os.Interrupt, syscall.SIGTERM)
	<-quitSignal

	// If the command wipe flag is set, remove all existing commands before shutting down the bot
	if commandWipe {
		log.Println("Command wipe flag is set, removing all existing commands...")
		for _, cmd := range registeredCommands {
			err := discord.ApplicationCommandDelete(discord.State.User.ID, "", cmd.ID)
			if err != nil {
				log.Printf("Cannot delete '%v' command: %v\n", cmd.Name, err)
			} else {
				log.Printf("Deleted command '%v'\n", cmd.Name)
			}
		}
		log.Println("All existing commands removed.")
	}

	// Close the Discord session when the program exits
	log.Println("Closing the discord session...")
	err = discord.Close()
	if err != nil {
		log.Println("Error closing Discord session:", err)
	}
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func ready(session *discordgo.Session, event *discordgo.Ready) {
	// Log that the bot is ready and set its status to the help command
	log.Printf("Melissa Bot is ready as: '%s#%s'\n", session.State.User.Username, session.State.User.Discriminator)
	session.UpdateGameStatus(0, "Type '/help' for more information!")
}

// This function will be called (due to AddHandler above) every time a new message
// is created on any channel that the bot has access to.
func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself or any other bots
	if message.Author.ID == session.State.User.ID || message.Author.Bot {
		return
	}

	// Log the content of the message if debug mode is enabled
	if debug {
		log.Printf("Received message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	// Route depending if the bot was mentioned in the message or not
	if message.Mentions != nil {
		for _, user := range message.Mentions {
			if user.ID == session.State.User.ID {
				mentionMessageCreate(session, message)
				return
			}
		}
	}

	// Route depending on whether the message was sent in a guild or in direct message
	if message.GuildID == "" {
		directMessageCreate(session, message)
		return
	} else {
		guildMessageCreate(session, message)
		return
	}
}

func mentionMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Remove the bot's mention from the start of the message content, if that is the case
	mention := fmt.Sprintf("<@%s>", session.State.User.ID)
	altMention := fmt.Sprintf("<@!%s>", session.State.User.ID)
	message.Content = strings.TrimPrefix(strings.TrimPrefix(message.Content, mention+" "), altMention+" ")

	// Answer the message properly if it starts with a known command
	if cmdName, found := commands.GetCmdNameFromMessage(message); found {
		if debug {
			log.Printf("Received command '%s' in message: '%s'\n", cmdName, message.Content)
		}

		if responseBuilder, ok := commands.Responses[cmdName]; ok {
			if _, err := session.ChannelMessageSend(message.ChannelID, responseBuilder()); err != nil {
				log.Printf("Failed to send response for '%s': %v\n", cmdName, err)
			}
			return
		}
	}

	// If we reached this point, simply reply to the message to let the user know the bot is responsive
	session.ChannelMessageSend(message.ChannelID, "Hello, I'm here!")
}

// This function will be called when the bot receives a message in a guild channel.
func guildMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// For now, simply ignore messages sent in guild channels
}

// This function will be called when the bot receives a message in a direct message channel.
func directMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Answer the message properly if it starts with a known command
	if cmdName, found := commands.GetCmdNameFromMessage(message); found {
		if debug {
			log.Printf("Received command '%s' in direct message: [%s#%s] '%s'\n",
				cmdName, message.Author.Username, message.Author.Discriminator, message.Content)
		}

		if responseBuilder, ok := commands.Responses[cmdName]; ok {
			if _, err := session.ChannelMessageSend(message.ChannelID, responseBuilder()); err != nil {
				log.Printf("Failed to send response for '%s': %v\n", cmdName, err)
			}
			return
		}
	}

	// If we reached this point, simply reply to the message to let the user know the bot is responsive
	session.ChannelMessageSend(message.ChannelID, "Hello!")
}
