package commands

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandDescriptionFallback(t *testing.T) {
	cmd := &command{
		Key: CmdHelp,
		Descriptions: map[discordgo.Locale]string{
			discordgo.EnglishUS:    "english",
			discordgo.PortugueseBR: "portuguese",
		},
	}

	if got := cmd.Description(discordgo.EnglishUS); got != cmd.Descriptions[discordgo.EnglishUS] {
		t.Fatalf("expected en-US description, got %q", got)
	}
	if got := cmd.Description(discordgo.PortugueseBR); got != cmd.Descriptions[discordgo.PortugueseBR] {
		t.Fatalf("expected pt-BR description, got %q", got)
	}
	if got := cmd.Description(discordgo.Japanese); got != cmd.Descriptions[discordgo.EnglishUS] {
		t.Fatalf("expected fallback (en-US) description, got %q", got)
	}
}

func TestApplicationCommandConversion(t *testing.T) {
	cmd := &command{
		Key: CommandKey("sample"),
		Descriptions: map[discordgo.Locale]string{
			discordgo.EnglishUS: "A command",
		},
	}

	app := cmd.ApplicationCommand()
	if app.Name != "sample" {
		t.Fatalf("unexpected command name: %s", app.Name)
	}
	if app.Description != "A command" {
		t.Fatalf("unexpected command description: %s", app.Description)
	}
	if app.DMPermission == nil || !*app.DMPermission {
		t.Fatalf("expected dm permission to be true")
	}
	if app.Contexts == nil || len(*app.Contexts) == 0 {
		t.Fatalf("expected contexts to be populated")
	}
	if app.IntegrationTypes == nil || len(*app.IntegrationTypes) == 0 {
		t.Fatalf("expected integration types to be populated")
	}
	if app.DescriptionLocalizations == nil {
		t.Fatalf("expected localized descriptions")
	}
}
