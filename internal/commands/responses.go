package commands

import (
	"fmt"
	"math/rand"

	"github.com/bwmarrin/discordgo"
)

// simpleResponse is a helper function that generates a response string for a command based on its ResponseTemplate and the provided locale.
// It returns an error if the command or its response template is nil, or if the default response template is missing.
func simpleResponse(cmd *command, locale discordgo.Locale) (string, error) {
	if cmd == nil {
		return "", ErrNilCommand
	}
	if cmd.ResponseTemplate == nil {
		return "", wrapCommandError(cmd.Key, ErrNilResponseTemplate)
	}
	if response, exists := textResolver().ResolveLocalizedText(cmd.ResponseTemplate, locale); exists {
		return response, nil
	}

	return "", wrapCommandError(cmd.Key, ErrMissingDefaultResponseTemplate)
}

// helpResponse generates a response for the help command,
// which lists all available commands and their descriptions based on the provided locale.
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

// rollResponse generates a response for a roll command,
// which simulates rolling a six-sided die and returns the result in the response string.
func rollResponse(cmd *command, locale discordgo.Locale) (string, error) {
	response, err := simpleResponse(cmd, locale)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(response, 1+rand.Intn(6)), nil
}
