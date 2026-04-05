package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Command names
// It's interesting to have them separate so they can be used in multiple places
const (
	CmdHelp = "help"
	CmdPing = "ping"
)

// Command descriptions and handlers
var (
	defaultDMPermission = true
	defaultContexts     = []discordgo.InteractionContextType{discordgo.InteractionContextGuild, discordgo.InteractionContextBotDM}
	defaultIntegrations = []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall}

	Definitions = []*discordgo.ApplicationCommand{
		{
			Name:             CmdHelp,
			Description:      "Provides information about the bot and its commands",
			DMPermission:     &defaultDMPermission,
			Contexts:         &defaultContexts,
			IntegrationTypes: &defaultIntegrations,
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.PortugueseBR: "Fornece informações sobre o bot e seus comandos",
			},
		},
		{
			Name:             CmdPing,
			Description:      "Pong!",
			DMPermission:     &defaultDMPermission,
			Contexts:         &defaultContexts,
			IntegrationTypes: &defaultIntegrations,
			DescriptionLocalizations: &map[discordgo.Locale]string{
				discordgo.PortugueseBR: "Pong!",
			},
		},
	}
	Handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		CmdHelp: cmdHandler,
		CmdPing: cmdHandler,
	}
	Responses = map[string]func() string{
		CmdHelp: helpResponse,
		CmdPing: pingResponse,
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
			Content: Responses[interaction.ApplicationCommandData().Name](),
		},
	}); err != nil {
		log.Printf("Failed to respond to /%s: %v\n", CmdHelp, err)
	}
}

// This functions returns a help message listing all available commands and their descriptions.
func helpResponse() string {
	response := "Hello! I'm Melissa Bot, your friendly Discord assistant. Here are some commands you can use:"
	for _, cmd := range Definitions {
		cmdBrief := fmt.Sprintf("/%s: %s", cmd.Name, cmd.Description)
		response = fmt.Sprintf("%s\n%s", response, cmdBrief)
	}

	return response
}

// Pong!
func pingResponse() string {
	return "Pong!"
}
