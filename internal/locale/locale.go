package locale

import (
	"github.com/bwmarrin/discordgo"
)

// TextResolver applies locale fallback rules using preconfigured desired/default locales.
type RegistryConfig struct {
	DesiredLocale discordgo.Locale
	DefaultLocale discordgo.Locale
}

// ResolveMessageLocale tries to resolve the locale of a message based on the message context and the user that sent it.
func ResolveMessageLocale(session *discordgo.Session, message *discordgo.MessageCreate) discordgo.Locale {
	// If the message was sent in a direct message channel, we try to get the locale of the user that sent the message
	if message.GuildID == "" {
		if user, err := session.User(message.Author.ID); err == nil && user != nil && user.Locale != "" {
			return discordgo.Locale(user.Locale)
		}
	}

	// Instead of the guild locale, we first try to get the locale of the guild owner
	// See: https://github.com/discord/discord-api-docs/discussions/4332
	if guild, err := session.Guild(message.GuildID); err == nil && guild != nil && guild.PreferredLocale != "" {
		if user, err := session.User(guild.OwnerID); err == nil && user != nil && user.Locale != "" {
			return discordgo.Locale(user.Locale)
		}

		// If the guild owner doesn't have a locale set, we fallback to the guild preferred locale
		return discordgo.Locale(guild.PreferredLocale)
	}

	// If we couldn't get any locale information from the message, we fallback to EnglishUS
	return discordgo.EnglishUS
}

// ResolveLocalizedText applies the same locale fallback chain used across command descriptions and responses.
func (registry RegistryConfig) ResolveLocalizedText(texts map[discordgo.Locale]string, requestedLocale discordgo.Locale) (string, bool) {
	if texts == nil {
		return "", false
	}

	if text, ok := texts[requestedLocale]; ok {
		return text, true
	}
	if text, ok := texts[registry.DesiredLocale]; ok {
		return text, true
	}
	if text, ok := texts[registry.DefaultLocale]; ok {
		return text, true
	}

	return "", false
}
