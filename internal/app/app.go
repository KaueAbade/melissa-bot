package app

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/KaueAbade/melissa-bot/internal/commands"
	"github.com/KaueAbade/melissa-bot/internal/locale"
	"github.com/KaueAbade/melissa-bot/internal/router"
	"github.com/bwmarrin/discordgo"
)

var appRuntimeInstance *Runtime
var newSession = discordgo.New
var setupSessionFatal = log.Fatal

// config defines the application configuration loaded from environment variables.
type Config struct {
	DiscordToken  string
	CommandWipe   bool
	Debug         bool
	DesiredLocale discordgo.Locale
}

// Runtime encapsulates the state and behavior of the bot application, including the Discord session,
type Runtime struct {
	session            *discordgo.Session
	registeredCommands []*discordgo.ApplicationCommand
	commandWipe        bool
	debug              bool
	getCommands        func() []*discordgo.ApplicationCommand
	executeFromContent func(string, discordgo.Locale) (string, error)
	executeFallback    func(discordgo.Locale) (string, error)
	openSession        func(*discordgo.Session) error
	closeSession       func(*discordgo.Session) error
	registerCommand    func(*discordgo.Session, *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error)
	deleteCommand      func(*discordgo.Session, *discordgo.ApplicationCommand) error
	updateGameStatus   func(*discordgo.Session)
}

// GetRuntime returns the package-wide runtime instance used by compatibility wrappers.
func GetRuntime() *Runtime {
	if appRuntimeInstance == nil {
		return nil
	}

	return appRuntimeInstance
}

// setRuntime creates a new Runtime instance with the provided configurations
func setRuntime(config *Config) *Runtime {
	appRuntimeInstance = &Runtime{
		commandWipe: config.CommandWipe,
		debug:       config.Debug,
		executeFromContent: func(content string, locale discordgo.Locale) (string, error) {
			return commands.GetRegistry().ExecuteFromContent(content, locale)
		},
		executeFallback: func(locale discordgo.Locale) (string, error) {
			return commands.GetRegistry().ExecuteFromKey(commands.Hello, locale)
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

	return appRuntimeInstance
}

// setupSession creates the Discord session and registers event handlers.
func SetupSession(config *Config) {
	var err error

	// Create a new Discord session using the provided bot token
	log.Println("Starting Melissa Bot...")
	runtime := setRuntime(config)
	runtime.session, err = newSession("Bot " + config.DiscordToken)
	if err != nil {
		setupSessionFatal(err)
	}

	// Specify the intents for the bot
	// For now it is simply required to listen for messages in order to reply to them
	log.Println("Setting the bots intents as GuildMessages, DirectMessages and MessageContent")
	runtime.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Register the desired functions as callbacks for events
	log.Println("Setting event handlers for ready, message creation and interactions")
	runtime.session.AddHandler(runtime.Ready)
	runtime.session.AddHandler(runtime.MessageCreate)
	runtime.session.AddHandler(runtime.InteractionCreate)
}

// Runtime.InteractionCreate is called whenever a slash command interaction is received.
func (app *Runtime) InteractionCreate(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	commands.GetRegistry().HandleInteraction(session, interaction)
}

// Runtime.Run opens the session and starts listening for events. It returns an error if the session is not properly initialized.
func (app *Runtime) Run() error {
	if app == nil || app.session == nil {
		return errors.New("runtime not initialized")
	}

	// Open a websocket connection to Discord and begin listening
	log.Println("Creating a websocket connection with the provided token")
	if err := app.openSession(app.session); err != nil {
		return err
	}

	return nil
}

// Runtime.Ready is called when the bot has successfully connected to Discord and is ready to operate.
func (app *Runtime) Ready(session *discordgo.Session, event *discordgo.Ready) {
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

// Runtime.Shutdown removes registered commands when requested and closes the session.
func (app *Runtime) Shutdown() {
	if app == nil || app.session == nil {
		return
	}

	// If the command wipe flag is set, remove all existing commands before shutting down the bot
	if app.commandWipe {
		log.Println("Command wipe flag is set, removing all existing commands...")
		for _, cmd := range app.registeredCommands {
			err := app.deleteCommand(app.session, cmd)
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
	if err := app.closeSession(app.session); err != nil {
		log.Println("Error closing Discord session:", err)
	}
}

// Runtime.MessageCreate is called whenever a new message is created in any channel the bot has access to.
// It routes the message to the appropriate handler based on its context (mention, direct message, or guild message).
func (app *Runtime) MessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself or any other bots
	if message.Author.ID == session.State.User.ID || message.Author.Bot {
		return
	}

	switch router.ResolveMessageRoute(session, message) {
	case router.RouteMention:
		app.mentionMessageCreate(session, message)
		return
	case router.RouteDirect:
		app.directMessageCreate(session, message)
		return
	default:
		app.guildMessageCreate(session, message)
		return
	}
}

// Runtime.mentionMessageCreate is called when the bot receives a message that mentions it.
func (app *Runtime) mentionMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
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

// Runtime.guildMessageCreate is called when the bot receives a message in a guild channel.
// For now, it simply ignores those messages.
func (app *Runtime) guildMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// For now, simply ignore messages sent in guild channels

	// Log the content of the message if debug mode is enabled
	if app.debug {
		log.Printf("Received guild message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}
}

// Runtime.directMessageCreate is called when the bot receives a direct message.
// It tries to resolve the command from the message content and sends the response to the channel where the message was sent.
func (app *Runtime) directMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Log the content of the message if debug mode is enabled
	if app.debug {
		log.Printf("Received direct message: [%s#%s] '%s'\n",
			message.Author.Username, message.Author.Discriminator, message.Content)
	}

	app.respondToMessage(session, message)
}

// Runtime.respondToMessage tries to resolve a command from the message content
// and sends the response to the channel where the message was sent.
func (app *Runtime) respondToMessage(session *discordgo.Session, message *discordgo.MessageCreate) {
	locale := locale.ResolveMessageLocale(session, message)

	// Try to resolve the command from the message content, if that fails we fallback to the default command
	response, err := app.executeFromContent(message.Content, locale)
	if err != nil {
		if errors.Is(err, commands.ErrCommandNotFound) {
			log.Printf("Failed to resolve command from message content: '%s'\n", message.Content)
		}
		log.Printf("Failed to resolve command: %v\n", err)

		response, err = app.executeFallback(locale)
		if err != nil {
			log.Printf("Failed to resolve fallback command '%s': %v\n", commands.Hello, err)
			return
		}
	}

	if _, sendErr := session.ChannelMessageSend(message.ChannelID, response); sendErr != nil {
		log.Printf("Failed to send response: %v\n", sendErr)
	}
}
