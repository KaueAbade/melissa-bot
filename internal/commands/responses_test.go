package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestSimpleResponseUsesLocaleAndFallback(t *testing.T) {
	command := &command{
		Key: Hello,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS:    "Hello!",
			discordgo.PortugueseBR: "Olá!",
		},
	}

	if got, err := simpleResponse(command, discordgo.EnglishUS); err != nil || got != command.ResponseTemplate[discordgo.EnglishUS] {
		t.Fatalf("expected en-US response, got %q", got)
	}
	if got, err := simpleResponse(command, discordgo.PortugueseBR); err != nil || got != command.ResponseTemplate[discordgo.PortugueseBR] {
		t.Fatalf("expected pt-BR response, got %q", got)
	}
	if got, err := simpleResponse(command, discordgo.Japanese); err != nil || got != command.ResponseTemplate[discordgo.EnglishUS] {
		t.Fatalf("expected default (en-US) response, got %q", got)
	}
}

func TestSimpleResponseReturnsErrorForNilTemplate(t *testing.T) {
	command := &command{Key: Hello}

	if _, err := simpleResponse(command, discordgo.EnglishUS); err == nil {
		t.Fatalf("expected error when response template is nil")
	}
}

func TestSimpleResponseReturnsErrorForNilCommand(t *testing.T) {
	if _, err := simpleResponse(nil, discordgo.EnglishUS); err == nil {
		t.Fatalf("expected error when command is nil")
	}
}

func TestSimpleResponseFallsBackToDesiredLocale(t *testing.T) {
	withTemporaryDesiredLocale(t, discordgo.PortugueseBR)

	command := &command{
		Key: Hello,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS:    "Hello!",
			discordgo.PortugueseBR: "Ola!",
		},
	}

	got, err := simpleResponse(command, discordgo.Japanese)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Ola!" {
		t.Fatalf("expected fallback to desired locale response, got %q", got)
	}
}

func TestSimpleResponseFallsBackToEnglishWhenDesiredMissing(t *testing.T) {
	withTemporaryDesiredLocale(t, discordgo.PortugueseBR)

	command := &command{
		Key: Hello,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS: "Hello!",
		},
	}

	got, err := simpleResponse(command, discordgo.Japanese)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello!" {
		t.Fatalf("expected fallback to English response, got %q", got)
	}
}

func TestHelpResponseIncludesCommands(t *testing.T) {
	command := &command{
		Key: Help,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS: "Commands:",
		},
	}

	got, err := helpResponse(command, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error in help response: %v", err)
	}
	if !strings.HasPrefix(got, "Commands:") {
		t.Fatalf("expected help prefix in response, got %q", got)
	}

	for _, cmdDef := range GetRegistry().getCommandDefinitionsSnapshot() {
		line := fmt.Sprintf("/%s:", cmdDef.Key)
		if !strings.Contains(got, line) {
			t.Fatalf("expected help response to include %q, got %q", line, got)
		}
	}
}

func TestHelpResponseUsesCommandDefinitionsDirectly(t *testing.T) {
	registry := withTemporaryRegistry(t, []*command{
		{
			Key: Help,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "Help",
			},
			ResponseBuilder: helpResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "Commands:",
			},
		},
		{
			Key: CommandKey("ghost"),
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "Ghost",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "boo",
			},
		},
	})

	helpCmd, exists := registry.getCmdFromKey(Help)
	if !exists {
		t.Fatalf("expected help command lookup")
	}

	got, err := helpResponse(helpCmd, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error in help response: %v", err)
	}
	if !strings.Contains(got, "/help:") {
		t.Fatalf("expected help command line in response, got %q", got)
	}
	if !strings.Contains(got, "/ghost:") {
		t.Fatalf("expected help response to include ghost command definition, got %q", got)
	}
}

func TestRollResponseRange(t *testing.T) {
	command := &command{
		Key: Roll,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS: "You rolled a %d!",
		},
	}

	for i := 0; i < 200; i++ {
		got, err := rollResponse(command, discordgo.EnglishUS)
		if err != nil {
			t.Fatalf("unexpected error in roll response: %v", err)
		}
		var value int
		if _, err := fmt.Sscanf(got, "You rolled a %d!", &value); err != nil {
			t.Fatalf("unable to parse roll response %q: %v", got, err)
		}
		if value < 1 || value > 6 {
			t.Fatalf("roll out of range: %d", value)
		}
	}
}

func TestHelpResponseReturnsErrorForInvalidTemplate(t *testing.T) {
	command := &command{
		Key:              Help,
		ResponseTemplate: map[discordgo.Locale]string{},
	}

	if _, err := helpResponse(command, discordgo.EnglishUS); err == nil {
		t.Fatalf("expected help response error for missing default template")
	}
}

func TestRollResponseReturnsErrorForNilCommand(t *testing.T) {
	if _, err := rollResponse(nil, discordgo.EnglishUS); err == nil {
		t.Fatalf("expected roll response error for nil command")
	}
}
