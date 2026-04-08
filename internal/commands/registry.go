package commands

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

var commandRegistryInstance *Registry

// Registry defines the structure for managing bot commands, including their definitions, desired locale, and methods for executing commands and handling interactions.
type Registry struct {
	mu            sync.RWMutex
	desiredLocale discordgo.Locale
	commands      map[CommandKey]*command
	commandsDef   []*command
}

// newRegistry creates a new Registry instance with the provided command definitions and desired locale.
func newRegistry(definitions []*command, desiredLocale discordgo.Locale) *Registry {
	registry := &Registry{
		desiredLocale: desiredLocale,
		commands:      make(map[CommandKey]*command, len(definitions)),
		commandsDef:   append([]*command(nil), definitions...),
	}

	for _, cmd := range definitions {
		if cmd != nil {
			normalizedKey := normalizeCommandKey(cmd.Key.String())
			if normalizedKey != "" {
				registry.commands[normalizedKey] = cmd
			}
		}
	}

	return registry
}

// GetRegistry returns the package-wide registry instance used by compatibility wrappers.
func GetRegistry() *Registry {
	if commandRegistryInstance == nil {
		commandRegistryInstance = newRegistry(nil, discordgo.EnglishUS)
	}
	return commandRegistryInstance
}

// init initializes the command registry with the default command definitions and desired locale.
// This is called automatically when the package is imported.
func init() {
	commandRegistryInstance = newRegistry(getCommandDefinitions(), defaultLocale)
}

// Registry.SetDesiredLocale updates the desired locale for the registry, which is used for localizing command descriptions and responses.
func (registry *Registry) SetDesiredLocale(locale discordgo.Locale) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.desiredLocale = locale
}

// Registry.GetDesiredLocale returns the currently set desired locale for the registry, which is used for localizing command descriptions and responses.
func (registry *Registry) GetDesiredLocale() discordgo.Locale {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.desiredLocale
}

// Registry.getDefaultLocale returns the default locale used for command descriptions and responses when no specific locale is found.
func (registry *Registry) getDefaultLocale() discordgo.Locale {
	return defaultLocale
}

// Registry.getCommandDefinitionsSnapshot returns a snapshot of all registered command definitions.
func (registry *Registry) getCommandDefinitionsSnapshot() []*command {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]*command(nil), registry.commandsDef...)
}

// Registry.getCmdFromContent attempts to resolve a command from the given content string,
// which is typically the message content.
func (registry *Registry) getCmdFromContent(content string) (*command, bool) {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return nil, false
	}

	return registry.getCmdFromName(fields[0])
}

// Registry.getCmdFromName attempts to resolve a command from the given name string,
// which is typically the first word of the message content.
func (registry *Registry) getCmdFromName(name string) (*command, bool) {
	key := normalizeCommandKey(name)
	if key == "" {
		return nil, false
	}

	return registry.getCmdFromKey(key)
}

// Registry.getCmdFromKey attempts to resolve a command from the given CommandKey.
func (registry *Registry) getCmdFromKey(key CommandKey) (*command, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if cmd, exists := registry.commands[key]; exists {
		return cmd, true
	}
	return nil, false
}

// Registry.ExecuteFromKey resolves a command by its key and returns its localized response.
func (registry *Registry) ExecuteFromKey(key CommandKey, locale discordgo.Locale) (string, error) {
	cmd, exists := registry.getCmdFromKey(key)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// Registry.ExecuteFromName resolves a command by its name and returns its localized response.
func (registry *Registry) ExecuteFromName(name string, locale discordgo.Locale) (string, error) {
	cmd, exists := registry.getCmdFromName(name)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// Registry.ExecuteFromContent resolves a command from content and returns its localized response.
func (registry *Registry) ExecuteFromContent(content string, locale discordgo.Locale) (string, error) {
	cmd, exists := registry.getCmdFromContent(content)
	if !exists {
		return "", ErrCommandNotFound
	}

	return cmd.Response(locale)
}

// Registry.GetApplicationCommands returns all registered commands in Discord API format.
func (registry *Registry) GetApplicationCommands() []*discordgo.ApplicationCommand {
	definitions := registry.getCommandDefinitionsSnapshot()
	applicationCommands := make([]*discordgo.ApplicationCommand, 0, len(definitions))
	for _, cmd := range definitions {
		applicationCommands = append(applicationCommands, cmd.applicationCommand(registry.GetDesiredLocale()))
	}
	return applicationCommands
}

// Registry.HandleInteraction is the main entry point for handling incoming Discord interactions.
// It resolves the command from the interaction data and sends the appropriate response back to Discord.
func (registry *Registry) HandleInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	cmdName := interaction.ApplicationCommandData().Name
	response, err := registry.ExecuteFromName(cmdName, interaction.Locale)
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

// Registry.ValidateCommands checks that all command definitions are structurally valid.
func (registry *Registry) ValidateCommands() error {
	definitions := registry.getCommandDefinitionsSnapshot()
	seen := map[CommandKey]struct{}{}
	for _, cmd := range definitions {
		if cmd == nil {
			return fmt.Errorf("nil command found in registry")
		}
		if cmd.Key == "" {
			return fmt.Errorf("command with empty key found")
		}
		normalizedKey := normalizeCommandKey(cmd.Key.String())
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
