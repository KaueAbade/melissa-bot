package commands

import (
	"github.com/bwmarrin/discordgo"
)

// command represents a bot command with its key, descriptions, response builder, and response templates.
type command struct {
	Key              CommandKey
	Descriptions     map[discordgo.Locale]string
	ResponseBuilder  func(c *command, l discordgo.Locale) (string, error)
	ResponseTemplate map[discordgo.Locale]string
}

// command.Description returns the localized description for the command based on the provided locale.
// If no description is found for the locale, it returns an empty string.
func (command *command) Description(locale discordgo.Locale) string {
	if description, exists := textResolver().ResolveLocalizedText(command.Descriptions, locale); exists {
		return description
	}

	return ""
}

// command.Response generates the response for the command based on the provided locale.
// It uses the ResponseBuilder function to create the response, passing the command and locale as arguments.
// If the command or ResponseBuilder is nil, it returns an error.
func (command *command) Response(locale discordgo.Locale) (string, error) {
	if command == nil {
		return "", ErrNilCommand
	}
	if command.ResponseBuilder == nil {
		return "", wrapCommandError(command.Key, ErrMissingResponseBuilder)
	}
	return command.ResponseBuilder(command, locale)
}

// command.ApplicationCommand converts the command into a discordgo.ApplicationCommand struct,
// which can be registered with the Discord API.
func (command *command) ApplicationCommand() *discordgo.ApplicationCommand {
	return command.applicationCommand(GetRegistry().GetDesiredLocale())
}

// command.applicationCommand is an internal method that converts the command into a discordgo.ApplicationCommand struct,
// using the provided desiredLocale for localization of the description.
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
