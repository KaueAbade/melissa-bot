package main

import (
	"errors"
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
	discord            *discordgo.Session
	discordToken       string
	registeredCommands []*discordgo.ApplicationCommand

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

	// Validate command consistency at startup
	if err := commands.ValidateCommands(); err != nil {
		log.Fatalf("Command validation failed: %v", err)
	}
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
		commands.HandleInteraction(session, interaction)
	})
}

func main() {
	// Open a websocket connection to Discord and begin listening
	log.Println("Creating a websocket connection with the provided token")
	err := discord.Open()
	if err != nil {
		log.Fatal(err)
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

	// Register the commands and their handlers
	applicationCommands := commands.GetApplicationCommands()
	registeredCommands = make([]*discordgo.ApplicationCommand, len(applicationCommands))
	for i, cmd := range applicationCommands {
		ccmd, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", cmd)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v\n", cmd.Name, err)
		}
		if debug {
			log.Printf("Registered command '%v'\n", cmd.Name)
		}
		registeredCommands[i] = ccmd
	}
}

// This function will be called (due to AddHandler above) every time a new message
// is created on any channel that the bot has access to.
func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself or any other bots
	if message.Author.ID == session.State.User.ID || message.Author.Bot {
		return
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

// This function will be called when the bot receives a message that mentions it.
func mentionMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Remove the bot's mention from the start of the message content, if that is the case
	mention := fmt.Sprintf("<@%s>", session.State.User.ID)
	altMention := fmt.Sprintf("<@!%s>", session.State.User.ID)
	message.Content = strings.TrimPrefix(strings.TrimPrefix(message.Content, mention+" "), altMention+" ")

	// Log the content of the message if debug mode is enabled
	if debug {
		log.Printf("Received mention message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	respondToMessage(session, message)
}

// This function will be called when the bot receives a message in a guild channel.
func guildMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// For now, simply ignore messages sent in guild channels

	// Log the content of the message if debug mode is enabled
	if debug {
		log.Printf("Received guild message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}
}

// This function will be called when the bot receives a message in a direct message channel.
func directMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if debug {
		log.Printf("Received direct message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	respondToMessage(session, message)
}

// As messages don't provide locale information, this function tries to resolve the most appropriate one
func resolveMessageLocale(session *discordgo.Session, message *discordgo.MessageCreate) discordgo.Locale {
	// If the message was sent in a direct message channel, we try to get the locale of the user that sent the message
	if message.GuildID == "" {
		if user, err := session.User(message.Author.ID); err == nil && user != nil && user.Locale != "" {
			return discordgo.Locale(user.Locale)
		}
	}

	// Instead of the guild locale, we first try to get the locale of the guild owner
	// See: https://github.com/discord/discord-api-docs/discussions/4332
	if guild, err := session.Guild(message.GuildID); err == nil && guild != nil && guild.PreferredLocale != "" {
		if user, err := session.User(guild.OwnerID); err == nil && user != nil && user.Locale != "" {
			return discordgo.Locale(user.Locale)
		}

		// If the guild owner doesn't have a locale set, we fallback to the guild preferred locale
		return discordgo.Locale(guild.PreferredLocale)
	}

	// If we couldn't get any locale information from the message, we fallback to EnglishUS
	return discordgo.EnglishUS
}

// This function tries to resolve the command from the message content
// and sends the response to the channel where the message was sent
func respondToMessage(session *discordgo.Session, message *discordgo.MessageCreate) {
	locale := resolveMessageLocale(session, message)

	// Try to resolve the command from the message content, if that fails we fallback to the default command
	response, err := commands.ExecuteFromContent(message.Content, locale)
	if err != nil {
		if errors.Is(err, commands.ErrCommandNotFound) {
			log.Printf("Failed to resolve command from message content: '%s'\n", message.Content)
		} else {
			log.Printf("Failed to resolve command: %v\n", err)
		}

		response, err = commands.ExecuteFromKey(commands.CmdHello, locale)
		if err != nil {
			log.Printf("Failed to resolve fallback command '%s': %v\n", commands.CmdHello, err)
			return
		}
	}

	if _, sendErr := session.ChannelMessageSend(message.ChannelID, response); sendErr != nil {
		log.Printf("Failed to send response: %v\n", sendErr)
	}
}
