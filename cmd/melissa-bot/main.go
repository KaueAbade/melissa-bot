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

type appRuntime struct {
	discord            *discordgo.Session
	registeredCommands []*discordgo.ApplicationCommand
	commandRegistry    *commands.Registry
	commandWipe        bool
	debug              bool
	getCommands        func() []*discordgo.ApplicationCommand
	execFromContent    func(content string, locale discordgo.Locale) (string, error)
	execFromKey        func(key commands.CommandKey, locale discordgo.Locale) (string, error)
	openSession        func(*discordgo.Session) error
	closeSession       func(*discordgo.Session) error
	registerCommand    func(*discordgo.Session, *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error)
	deleteCommand      func(*discordgo.Session, *discordgo.ApplicationCommand) error
	updateGameStatus   func(*discordgo.Session)
}

type appConfig struct {
	discordToken  string
	commandWipe   bool
	debug         bool
	defaultLocale discordgo.Locale
}

type messageRoute int

const (
	routeGuild messageRoute = iota
	routeDirect
	routeMention
)

var runtime *appRuntime

func newAppRuntime(commandWipe bool, debug bool, registry *commands.Registry) *appRuntime {
	if registry == nil {
		registry = commands.GetRegistry()
	}

	return &appRuntime{
		commandRegistry: registry,
		commandWipe:     commandWipe,
		debug:           debug,
		getCommands:     registry.GetApplicationCommands,
		execFromContent: func(content string, locale discordgo.Locale) (string, error) {
			return registry.ExecuteFromContent(content, locale)
		},
		execFromKey: func(key commands.CommandKey, locale discordgo.Locale) (string, error) {
			return registry.ExecuteFromKey(key, locale)
		},
		openSession: func(session *discordgo.Session) error {
			return session.Open()
		},
		closeSession: func(session *discordgo.Session) error {
			return session.Close()
		},
		registerCommand: func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
			return session.ApplicationCommandCreate(session.State.User.ID, "", cmd)
		},
		deleteCommand: func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) error {
			return session.ApplicationCommandDelete(session.State.User.ID, "", cmd.ID)
		},
		updateGameStatus: func(session *discordgo.Session) {
			session.UpdateGameStatus(0, "Type '/help' for more information!")
		},
	}
}

// Get bot configurations
func init() {
	log.Println("Setting up Melissa Bot...")
	config, err := loadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	registry := commands.GetRegistry()
	registry.SetDesiredLocale(config.defaultLocale)

	// Validate command consistency at startup
	if err := registry.ValidateCommands(); err != nil {
		log.Fatalf("Command validation failed: %v", err)
	}

	runtime = newAppRuntime(config.commandWipe, config.debug, registry)
	runtime.setupSession(config.discordToken)
}

func loadConfigFromEnv() (*appConfig, error) {
	// Get the bot token from an environment variable
	discordToken := env.GetStr("DISCORD_BOT_TOKEN", "")
	if discordToken == "" {
		return nil, errors.New("No token provided\nIt is necessary to set the DISCORD_BOT_TOKEN environment variable")
	}

	return &appConfig{
		discordToken:  discordToken,
		commandWipe:   env.GetBool("WIPE_COMMANDS_ON_EXIT", false),
		debug:         env.GetBool("DEBUG", false),
		defaultLocale: discordgo.Locale(env.GetStr("LOCALE", string(discordgo.EnglishUS))),
	}, nil
}

// setupSession creates the Discord session and registers event handlers.
func (app *appRuntime) setupSession(discordToken string) {
	var err error

	// Create a new Discord session using the provided bot token
	log.Println("Starting Melissa Bot...")
	app.discord, err = discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatal(err)
	}

	// Specify the intents for the bot
	// For now it is simply required to listen for messages in order to reply to them
	log.Println("Setting the bots intents as GuildMessages, DirectMessages and MessageContent")
	app.discord.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Register the desired functions as callbacks for events
	log.Println("Setting event handlers for ready, message creation and interactions")
	app.discord.AddHandler(app.ready)
	app.discord.AddHandler(app.messageCreate)
	app.discord.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		app.commandRegistry.HandleInteraction(session, interaction)
	})
}

func main() {
	if err := run(runtime, signal.Notify); err != nil {
		log.Fatal(err)
	}
}

func run(app *appRuntime, notify func(c chan<- os.Signal, sig ...os.Signal)) error {
	if app == nil || app.discord == nil {
		return errors.New("runtime not initialized")
	}

	// Open a websocket connection to Discord and begin listening
	log.Println("Creating a websocket connection with the provided token")
	if err := app.openSession(app.discord); err != nil {
		return err
	}

	// Wait here until a sigterm is received
	quitSignal := make(chan os.Signal, 1)
	notify(quitSignal, os.Interrupt, syscall.SIGTERM)
	<-quitSignal
	app.shutdown()

	return nil
}

// shutdown removes registered commands when requested and closes the session.
func (app *appRuntime) shutdown() {
	if app == nil || app.discord == nil {
		return
	}

	// If the command wipe flag is set, remove all existing commands before shutting down the bot
	if app.commandWipe {
		log.Println("Command wipe flag is set, removing all existing commands...")
		for _, cmd := range app.registeredCommands {
			err := app.deleteCommand(app.discord, cmd)
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
	if err := app.closeSession(app.discord); err != nil {
		log.Println("Error closing Discord session:", err)
	}
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func (app *appRuntime) ready(session *discordgo.Session, event *discordgo.Ready) {
	// Log that the bot is ready and set its status to the help command
	log.Printf("Melissa Bot is ready as: '%s#%s'\n", session.State.User.Username, session.State.User.Discriminator)
	app.updateGameStatus(session)

	// Register the commands and their handlers
	applicationCommands := app.getCommands()
	app.registeredCommands = make([]*discordgo.ApplicationCommand, 0, len(applicationCommands))
	for _, cmd := range applicationCommands {
		ccmd, err := app.registerCommand(session, cmd)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v\n", cmd.Name, err)
		}
		if app.debug {
			log.Printf("Registered command '%v'\n", cmd.Name)
		}
		app.registeredCommands = append(app.registeredCommands, ccmd)
	}
}

// This function will be called (due to AddHandler above) every time a new message
// is created on any channel that the bot has access to.
func (app *appRuntime) messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself or any other bots
	if message.Author.ID == session.State.User.ID || message.Author.Bot {
		return
	}

	switch resolveMessageRoute(session, message) {
	case routeMention:
		app.mentionMessageCreate(session, message)
		return
	case routeDirect:
		app.directMessageCreate(session, message)
		return
	default:
		app.guildMessageCreate(session, message)
		return
	}
}

func resolveMessageRoute(session *discordgo.Session, message *discordgo.MessageCreate) messageRoute {
	if message.Mentions != nil {
		for _, user := range message.Mentions {
			if user.ID == session.State.User.ID {
				return routeMention
			}
		}
	}

	if message.GuildID == "" {
		return routeDirect
	}

	return routeGuild
}

// This function will be called when the bot receives a message that mentions it.
func (app *appRuntime) mentionMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Remove the bot's mention from the start of the message content, if that is the case
	mention := fmt.Sprintf("<@%s>", session.State.User.ID)
	altMention := fmt.Sprintf("<@!%s>", session.State.User.ID)
	message.Content = strings.TrimPrefix(strings.TrimPrefix(message.Content, mention+" "), altMention+" ")

	// Log the content of the message if debug mode is enabled
	if app.debug {
		log.Printf("Received mention message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	app.respondToMessage(session, message)
}

// This function will be called when the bot receives a message in a guild channel.
func (app *appRuntime) guildMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// For now, simply ignore messages sent in guild channels

	// Log the content of the message if debug mode is enabled
	if app.debug {
		log.Printf("Received guild message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}
}

// This function will be called when the bot receives a message in a direct message channel.
func (app *appRuntime) directMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if app.debug {
		log.Printf("Received direct message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	app.respondToMessage(session, message)
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
func (app *appRuntime) respondToMessage(session *discordgo.Session, message *discordgo.MessageCreate) {
	locale := resolveMessageLocale(session, message)

	// Try to resolve the command from the message content, if that fails we fallback to the default command
	response, err := app.execFromContent(message.Content, locale)
	if err != nil {
		logMessageCommandResolutionError(message.Content, err)

		response, err = app.execFromKey(commands.Hello, locale)
		if err != nil {
			log.Printf("Failed to resolve fallback command '%s': %v\n", commands.Hello, err)
			return
		}
	}

	if _, sendErr := session.ChannelMessageSend(message.ChannelID, response); sendErr != nil {
		log.Printf("Failed to send response: %v\n", sendErr)
	}
}

func logMessageCommandResolutionError(content string, err error) {
	if errors.Is(err, commands.ErrCommandNotFound) {
		log.Printf("Failed to resolve command from message content: '%s'\n", content)
		return
	}

	log.Printf("Failed to resolve command: %v\n", err)
}
