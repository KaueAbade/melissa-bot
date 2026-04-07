package commands

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// CommandKey is the typed identifier for a bot command keyword.
type CommandKey string

func (key CommandKey) String() string {
	return string(key)
}

// Default command properties, can be overridden by individual commands if needed
var (
	defaultDMPermission = true
	defaultContexts     = []discordgo.InteractionContextType{discordgo.InteractionContextGuild, discordgo.InteractionContextBotDM}
	defaultIntegrations = []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall}
	defaultLocale       = discordgo.EnglishUS
	desiredLocale       = discordgo.EnglishUS
	commands            map[CommandKey]*command
	commandsDef         []*command
)

// Exported command keys for external callers.
const (
	Help  CommandKey = "help"
	Hello CommandKey = "hello"
	Ping  CommandKey = "ping"
	Roll  CommandKey = "roll"
)

func init() {
	// Command registry initialization
	commandsDef = []*command{
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

	// Build a command lookup map
	commands = make(map[CommandKey]*command, len(commandsDef))
	for _, cmd := range commandsDef {
		commands[cmd.Key] = cmd
	}
}

// SetLocale is a helper function to set the default locale
func SetDesiredLocale(locale discordgo.Locale) {
	// Update the default locale
	log.Printf("Setting desired locale to: %s\n", locale)
	desiredLocale = locale
}

// ExecuteFromKey resolves a command by key and returns its localized response.
func ExecuteFromKey(key CommandKey, locale discordgo.Locale) (string, error) {
	cmd, exists := getCmdFromKey(key)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// ExecuteFromName resolves a command by name and returns its localized response.
func ExecuteFromName(name string, locale discordgo.Locale) (string, error) {
	cmd, exists := getCmdFromName(name)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// ExecuteFromContent resolves a command from content and returns its localized response.
func ExecuteFromContent(content string, locale discordgo.Locale) (string, error) {
	cmd, exists := getCmdFromContent(content)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// ApplicationCommands returns a slice of discordgo.ApplicationCommand structs for all registered commands,
// which can be used for registration with Discord.
func GetApplicationCommands() []*discordgo.ApplicationCommand {
	applicationCommands := make([]*discordgo.ApplicationCommand, 0, len(commandsDef))
	for _, cmd := range commandsDef {
		applicationCommands = append(applicationCommands, cmd.ApplicationCommand())
	}
	return applicationCommands
}

// HandleInteraction dispatches a slash command and responds to Discord.
func HandleInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	cmdName := interaction.ApplicationCommandData().Name
	response, err := ExecuteFromName(cmdName, interaction.Locale)
	if err != nil {
		if errors.Is(err, ErrCommandNotFound) {
			log.Printf("Failed to resolve /%s response\n", cmdName)
		} else {
			log.Printf("Failed to build response for /%s: %v\n", cmdName, err)
		}
		return
	}
	if err := session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: response},
	}); err != nil {
		log.Printf("Failed to respond to /%s: %v\n", cmdName, err)
	}
}

// ValidateCommands checks that all command definitions are structurally valid.
func ValidateCommands() error {
	seen := map[CommandKey]struct{}{}
	for _, cmd := range commandsDef {
		if cmd == nil {
			return fmt.Errorf("nil command found in registry")
		}
		if cmd.Key == "" {
			return fmt.Errorf("command with empty key found")
		}
		normalizedKey := CommandKey(strings.ToLower(strings.TrimSpace(cmd.Key.String())))
		if normalizedKey == "" {
			return fmt.Errorf("command with invalid normalized key: %q", cmd.Key)
		}
		if _, exists := seen[normalizedKey]; exists {
			return fmt.Errorf("duplicate command key found: %s", normalizedKey)
		}
		seen[normalizedKey] = struct{}{}
		if len(cmd.Descriptions) == 0 {
			return fmt.Errorf("command '%s' has no descriptions", normalizedKey)
		}
		if _, ok := cmd.Descriptions[defaultLocale]; !ok {
			return fmt.Errorf("command '%s' has no default locale description", normalizedKey)
		}

		if cmd.ResponseBuilder == nil {
			return fmt.Errorf("command '%s' missing response builder", normalizedKey)
		}
		if len(cmd.ResponseTemplate) == 0 {
			return fmt.Errorf("command '%s' has no responses", normalizedKey)
		}
		if _, ok := cmd.ResponseTemplate[defaultLocale]; !ok {
			return fmt.Errorf("command '%s' has no default locale response", normalizedKey)
		}
	}

	return nil
}

// getCmdKeyFromContent parses and validates the first token as a typed command key.
func getCmdFromContent(content string) (*command, bool) {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return nil, false
	}

	return getCmdFromName(fields[0])
}

// getCmdKeyFromName parses and validates the input string as a typed command key.
func getCmdFromName(name string) (*command, bool) {
	key := CommandKey(strings.ToLower(strings.TrimSpace(name)))
	if key == "" {
		return nil, false
	}

	return getCmdFromKey(key)
}

// getCmdFromKey looks up a Command struct by its CommandKey.
func getCmdFromKey(key CommandKey) (*command, bool) {
	if cmd, exists := commands[key]; exists {
		return cmd, true
	}
	return nil, false
}
