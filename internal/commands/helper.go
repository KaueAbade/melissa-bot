package commands

import (
	"strings"

	"github.com/KaueAbade/melissa-bot/internal/locale"
)

// normalizeCommandKey takes a command key string and normalizes it by trimming whitespace and converting it to lowercase.
func normalizeCommandKey(key string) CommandKey {
	return CommandKey(strings.ToLower(strings.TrimSpace(key)))
}

// textResolver creates a locale.RegistryConfig based on the desired and default locales from the command registry.
func textResolver() locale.RegistryConfig {
	registry := GetRegistry()
	return locale.RegistryConfig{
		DesiredLocale: registry.GetDesiredLocale(),
		DefaultLocale: registry.getDefaultLocale(),
	}
}
