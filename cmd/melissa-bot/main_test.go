package main

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestLoadConfigFromEnvMissingToken(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "")

	config, err := loadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected missing token error")
	}
	if config != nil {
		t.Fatalf("expected nil config when token is missing")
	}
	if !strings.Contains(err.Error(), "DISCORD_BOT_TOKEN") {
		t.Fatalf("expected error to mention DISCORD_BOT_TOKEN, got %q", err.Error())
	}
}

func TestLoadConfigFromEnvSuccess(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token-123")
	t.Setenv("WIPE_COMMANDS_ON_EXIT", "true")
	t.Setenv("DEBUG", "true")
	t.Setenv("LOCALE", string(discordgo.PortugueseBR))

	config, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if config.DiscordToken != "token-123" {
		t.Fatalf("unexpected token: %q", config.DiscordToken)
	}
	if !config.CommandWipe {
		t.Fatalf("expected command wipe to be true")
	}
	if !config.Debug {
		t.Fatalf("expected debug to be true")
	}
	if config.DesiredLocale != discordgo.PortugueseBR {
		t.Fatalf("unexpected desired locale: %s", config.DesiredLocale)
	}
}

func TestLoadConfigFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token-123")
	t.Setenv("WIPE_COMMANDS_ON_EXIT", "")
	t.Setenv("DEBUG", "")
	t.Setenv("LOCALE", "")

	config, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if config.CommandWipe {
		t.Fatalf("expected command wipe default to be false")
	}
	if config.Debug {
		t.Fatalf("expected debug default to be false")
	}
	if config.DesiredLocale != discordgo.EnglishUS {
		t.Fatalf("expected default locale %s, got %s", discordgo.EnglishUS, config.DesiredLocale)
	}
}

func TestLoadConfigFromEnvInvalidBooleansFallbackToFalse(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token-123")
	t.Setenv("WIPE_COMMANDS_ON_EXIT", "not-a-bool")
	t.Setenv("DEBUG", "invalid")
	t.Setenv("LOCALE", string(discordgo.Japanese))

	config, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if config.CommandWipe {
		t.Fatalf("expected invalid wipe value to fallback to false")
	}
	if config.Debug {
		t.Fatalf("expected invalid debug value to fallback to false")
	}
	if config.DesiredLocale != discordgo.Japanese {
		t.Fatalf("expected locale override to remain applied, got %s", config.DesiredLocale)
	}
}
