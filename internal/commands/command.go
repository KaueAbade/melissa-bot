package commands

import (
	"github.com/bwmarrin/discordgo"
)

// Command struct defines the properties of a bot command, including its name, descriptions in multiple locales
// a response builder function, and the response messages in multiple locales
type command struct {
	Key              CommandKey
	Descriptions     map[discordgo.Locale]string
	ResponseBuilder  func(cmd *command, l discordgo.Locale) (string, error)
	ResponseTemplate map[discordgo.Locale]string
}

// Command.Description enables getting the description of a command based on the locale,
// falling back to the default locale if the specific one is not available
func (command *command) Description(locale discordgo.Locale) string {
	if description, exists := resolveLocalizedText(command.Descriptions, locale); exists {
		return description
	}

	return ""
}

// Command.Response builds the localized command response.
func (command *command) Response(locale discordgo.Locale) (string, error) {
	if command == nil {
		return "", ErrNilCommand
	}
	if command.ResponseBuilder == nil {
		return "", wrapCommandError(command.Key, ErrMissingResponseBuilder)
	}
	return command.ResponseBuilder(command, locale)
}

// Command.ApplicationCommand converts a Command struct into a discordgo.ApplicationCommand
// struct for registration with Discord.
func (command *command) ApplicationCommand() *discordgo.ApplicationCommand {
	return command.applicationCommand(GetRegistry().getDesiredLocale())
}

func (command *command) applicationCommand(desiredLocale discordgo.Locale) *discordgo.ApplicationCommand {
	descriptions := command.Descriptions
	return &discordgo.ApplicationCommand{
		Name:                     command.Key.String(),
		Description:              command.Description(desiredLocale),
		DescriptionLocalizations: &descriptions,
		DMPermission:             &defaultDMPermission,
		Contexts:                 &defaultContexts,
		IntegrationTypes:         &defaultIntegrations,
	}
}
