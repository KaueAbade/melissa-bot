package commands

import (
	"github.com/bwmarrin/discordgo"
)

// resolveLocalizedText applies the same locale fallback chain used across command descriptions and responses.
func resolveLocalizedText(texts map[discordgo.Locale]string, locale discordgo.Locale) (string, bool) {
	if texts == nil {
		return "", false
	}

	if text, ok := texts[locale]; ok {
		return text, true
	}
	if text, ok := texts[GetRegistry().getDesiredLocale()]; ok {
		return text, true
	}
	if text, ok := texts[defaultLocale]; ok {
		return text, true
	}

	return "", false
}
