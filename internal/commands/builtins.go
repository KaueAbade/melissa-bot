package commands

import "github.com/bwmarrin/discordgo"

// Exported command keys for external callers.
type CommandKey string

const (
	Help  CommandKey = "help"
	Hello CommandKey = "hello"
	Ping  CommandKey = "ping"
	Roll  CommandKey = "roll"
)

// CommandKey.String returns the string representation of the CommandKey.
func (key CommandKey) String() string {
	return string(key)
}

// Default command properties, can be overridden by individual commands if needed
var (
	defaultDMPermission = true
	defaultContexts     = []discordgo.InteractionContextType{discordgo.InteractionContextGuild, discordgo.InteractionContextBotDM}
	defaultIntegrations = []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall}
	defaultLocale       = discordgo.EnglishUS
)

// getCommandDefinitions returns the list of all command definitions, which can be used to initialize the registry.
func getCommandDefinitions() []*command {
	return []*command{
		{
			Key: Help,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "Provides information about the bot and its commands",
				discordgo.PortugueseBR: "Fornece informações sobre o bot e seus comandos",
			},
			ResponseBuilder: helpResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "Here are some commands you can use:",
				discordgo.PortugueseBR: "Aqui estão alguns comandos que você pode usar:",
			},
		},
		{
			Key: Hello,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "Says hello to the user",
				discordgo.PortugueseBR: "Diz olá para o usuário",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "Hello! I'm Melissa Bot, your friendly Discord assistant.",
				discordgo.PortugueseBR: "Olá! Eu sou a Melissa Bot, sua assistente amigável do Discord.",
			},
		},
		{
			Key: Ping,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "Pong!",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "Pong!",
			},
		},
		{
			Key: Roll,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "Rolls a dice and returns the result",
				discordgo.PortugueseBR: "Joga um dado e retorna o resultado",
			},
			ResponseBuilder: rollResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS:    "You rolled a %d!",
				discordgo.PortugueseBR: "Você rolou um %d!",
			},
		},
	}
}
