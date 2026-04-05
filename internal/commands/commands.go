package commands

import (
	"fmt"

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
		CmdHelp: HelpHandler,
		CmdPing: PingHandler,
	}
)

// This should provide information about the bot and its commands.
func HelpHandler(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// Build the response message with the list of commands and their descriptions
	response := "Hello! I'm Melissa Bot, your friendly Discord assistant. Here are some commands you can use:"
	for _, cmd := range Definitions {
		cmdBrief := fmt.Sprintf("/%s: %s", cmd.Name, cmd.Description)
		response = fmt.Sprintf("%s\n%s", response, cmdBrief)
	}

	// Send the response back to the user
	session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
}

// This should respond with "Pong!" to test if the bot is responsive.
func PingHandler(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// Send the response back to the user
	session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong!",
		},
	})
}
