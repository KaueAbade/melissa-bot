package commands

import (
	"fmt"

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
	// Check if the command has a description for the requested locale, if not we fallback to the desired locale
	if description, exists := command.Descriptions[locale]; exists {
		return description
	}

	description, exists := command.Descriptions[desiredLocale]
	if !exists {
		description = command.Descriptions[defaultLocale]
	}
	return description
}

// Command.Response builds the localized command response.
func (command *command) Response(locale discordgo.Locale) (string, error) {
	if command.ResponseBuilder == nil {
		return "", fmt.Errorf("command '%s' missing response builder", command.Key)
	}
	return command.ResponseBuilder(command, locale)
}

// Command.ApplicationCommand converts a Command struct into a discordgo.ApplicationCommand
// struct for registration with Discord.
func (command *command) ApplicationCommand() *discordgo.ApplicationCommand {
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
