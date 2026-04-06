package commands

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Command names
// It's interesting to have them separate so they can be used in multiple places
const (
	CmdHelp  = "help"
	CmdHello = "hello"
	CmdPing  = "ping"
	CmdRoll  = "roll"

	defaultLocale = discordgo.EnglishUS
)

var localizedDescriptions = map[string]map[discordgo.Locale]string{
	CmdHelp: {
		discordgo.EnglishUS:    "Provides information about the bot and its commands",
		discordgo.PortugueseBR: "Fornece informações sobre o bot e seus comandos",
	},
	CmdHello: {
		discordgo.EnglishUS:    "Says hello to the user",
		discordgo.PortugueseBR: "Diz olá para o usuário",
	},
	CmdPing: {
		discordgo.EnglishUS: "Pong!",
	},
	CmdRoll: {
		discordgo.EnglishUS:    "Rolls a dice and returns the result",
		discordgo.PortugueseBR: "Joga um dado e retorna o resultado",
	},
}

var localizedResponses = map[string]map[discordgo.Locale]string{
	CmdHelp: {
		discordgo.EnglishUS:    "Here are some commands you can use:",
		discordgo.PortugueseBR: "Aqui estão alguns comandos que você pode usar:",
	},
	CmdHello: {
		discordgo.EnglishUS:    "Hello! I'm Melissa Bot, your friendly Discord assistant.",
		discordgo.PortugueseBR: "Olá! Eu sou a Melissa Bot, sua assistente amigável do Discord.",
	},
	CmdPing: {
		discordgo.EnglishUS: "Pong!",
	},
	CmdRoll: {
		discordgo.EnglishUS:    "You rolled a %d!",
		discordgo.PortugueseBR: "Você rolou um %d!",
	},
}

// Command descriptions and handlers
var (
	defaultDMPermission = true
	defaultContexts     = []discordgo.InteractionContextType{discordgo.InteractionContextGuild, discordgo.InteractionContextBotDM}
	defaultIntegrations = []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall}
	helpDescriptions    = localizedDescriptions[CmdHelp]
	helloDescriptions   = localizedDescriptions[CmdHello]
	pingDescriptions    = localizedDescriptions[CmdPing]
	rollDescriptions    = localizedDescriptions[CmdRoll]

	// Command definitions with their respective handlers and responses
	Definitions = []*discordgo.ApplicationCommand{
		{
			Name:                     CmdHelp,
			Description:              helpDescriptions[discordgo.EnglishUS],
			DescriptionLocalizations: &helpDescriptions,
			DMPermission:             &defaultDMPermission,
			Contexts:                 &defaultContexts,
			IntegrationTypes:         &defaultIntegrations,
		},
		{
			Name:                     CmdHello,
			Description:              helloDescriptions[discordgo.EnglishUS],
			DescriptionLocalizations: &helloDescriptions,
			DMPermission:             &defaultDMPermission,
			Contexts:                 &defaultContexts,
			IntegrationTypes:         &defaultIntegrations,
		},
		{
			Name:                     CmdPing,
			Description:              pingDescriptions[discordgo.EnglishUS],
			DescriptionLocalizations: &pingDescriptions,
			DMPermission:             &defaultDMPermission,
			Contexts:                 &defaultContexts,
			IntegrationTypes:         &defaultIntegrations,
		},
		{
			Name:                     CmdRoll,
			Description:              rollDescriptions[discordgo.EnglishUS],
			DescriptionLocalizations: &rollDescriptions,
			DMPermission:             &defaultDMPermission,
			Contexts:                 &defaultContexts,
			IntegrationTypes:         &defaultIntegrations,
		},
	}
	Handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		CmdHelp:  cmdHandler,
		CmdHello: cmdHandler,
		CmdPing:  cmdHandler,
		CmdRoll:  cmdHandler,
	}
	Responses = map[string]func(l discordgo.Locale) string{
		CmdHelp:  helpResponse,
		CmdHello: helloResponse,
		CmdPing:  pingResponse,
		CmdRoll:  rollResponse,
	}
)

// This function parses a command name from a message, checking if it starts with any of the defined commands
func GetCmdNameFromMessage(message *discordgo.MessageCreate) (cmdName string, found bool) {
	// Returns early if the message content is empty
	if message.Content == "" {
		return "", false
	}

	// Check if the message contains an valid command as a prefix
	// Turn first character to lowercase to make the command recognition case insensitive
	message.Content = strings.ToLower(message.Content[:1]) + message.Content[1:]
	for _, cmd := range Definitions {
		if strings.HasPrefix(message.Content, cmd.Name) {
			return cmd.Name, true
		}
	}
	return "", false
}

// This function is called when a command interaction is received. It looks up the appropriate response and sends it back to the user.
func cmdHandler(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if err := session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: Responses[interaction.ApplicationCommandData().Name](interaction.Locale),
		},
	}); err != nil {
		log.Printf("Failed to respond to /%s: %v\n", interaction.ApplicationCommandData().Name, err)
	}
}

// This functions returns a help message listing all available commands and their descriptions.
func helpResponse(locale discordgo.Locale) string {
	// Fallback all command descriptions if header is not found in the user's locale
	if _, exists := localizedResponses[CmdHelp][locale]; !exists {
		locale = defaultLocale
	}

	// Append each command and its description to the response
	response := localizedResponses[CmdHelp][locale]
	for _, cmd := range Definitions {
		if cmd.DescriptionLocalizations != nil {
			if description, exists := (*cmd.DescriptionLocalizations)[locale]; exists {
				response = fmt.Sprintf("%s\n/%s: %s", response, cmd.Name, description)
			} else {
				// Fallback to default locale if the command description is not localized in the user's locale
				response = fmt.Sprintf("%s\n/%s: %s", response, cmd.Name, cmd.Description)
			}
		}
	}

	return response
}

// Hello!
func helloResponse(locale discordgo.Locale) string {
	if response, exists := localizedResponses[CmdHello][locale]; exists {
		return response
	}
	return localizedResponses[CmdHello][defaultLocale]
}

// Pong!
func pingResponse(locale discordgo.Locale) string {
	if response, exists := localizedResponses[CmdPing][locale]; exists {
		return response
	}
	return localizedResponses[CmdPing][defaultLocale]
}

// Rolls a dice and returns the result
func rollResponse(locale discordgo.Locale) string {
	if response, exists := localizedResponses[CmdRoll][locale]; exists {
		return fmt.Sprintf(response, 1+rand.Intn(6))
	}
	return fmt.Sprintf(localizedResponses[CmdRoll][defaultLocale], 1+rand.Intn(6))
}

// ValidateCommands checks that all command definitions have corresponding handlers and response builders.
// This should be called at bot startup to catch configuration errors early.
func ValidateCommands() error {
	for _, cmd := range Definitions {
		// Check if handler exists
		if _, ok := Handlers[cmd.Name]; !ok {
			return fmt.Errorf("command '%s' defined but missing handler", cmd.Name)
		}

		// Check if response builder exists
		if _, ok := Responses[cmd.Name]; !ok {
			return fmt.Errorf("command '%s' defined but missing response builder", cmd.Name)
		}

		// Check if localization is present
		if cmd.DescriptionLocalizations == nil {
			log.Printf("Warning: command '%s' has no localization map\n", cmd.Name)
		}
	}
	return nil
}
