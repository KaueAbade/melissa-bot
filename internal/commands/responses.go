package commands

import (
	"fmt"
	"math/rand"

	"github.com/bwmarrin/discordgo"
)

// This function simply returns the response for a command based on the locale, without any dynamic content
func simpleResponse(cmd *command, locale discordgo.Locale) (string, error) {
	if cmd == nil {
		return "", ErrNilCommand
	}
	if cmd.ResponseTemplate == nil {
		return "", wrapCommandError(cmd.Key, ErrNilResponseTemplate)
	}
	if response, exists := resolveLocalizedText(cmd.ResponseTemplate, locale); exists {
		return response, nil
	}

	return "", wrapCommandError(cmd.Key, ErrMissingDefaultResponseTemplate)
}

// This functions returns a help message listing all available commands and their descriptions.
func helpResponse(cmd *command, locale discordgo.Locale) (string, error) {
	// Get the base response for the help command based on the locale
	response, err := simpleResponse(cmd, locale)
	if err != nil {
		return "", err
	}

	// Append each command and its description to the response
	for _, cmdDef := range GetRegistry().getCommandDefinitionsSnapshot() {
		cmdBrief := fmt.Sprintf("/%s: %s", cmdDef.Key, cmdDef.Description(locale))
		response = fmt.Sprintf("%s\n%s", response, cmdBrief)
	}

	return response, nil
}

// Rolls a dice and returns the result
func rollResponse(cmd *command, locale discordgo.Locale) (string, error) {
	response, err := simpleResponse(cmd, locale)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(response, 1+rand.Intn(6)), nil
}
