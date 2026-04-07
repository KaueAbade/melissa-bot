package commands

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestWrapCommandErrorWithNilError(t *testing.T) {
	if err := wrapCommandError(Hello, nil); err != nil {
		t.Fatalf("expected nil when wrapping nil error, got %v", err)
	}
}

func TestCommandResponseMissingBuilderReturnsTypedError(t *testing.T) {
	cmd := &command{Key: CommandKey("broken")}

	_, err := cmd.Response(discordgo.EnglishUS)
	if !errors.Is(err, ErrMissingResponseBuilder) {
		t.Fatalf("expected ErrMissingResponseBuilder, got %v", err)
	}
}

func TestSimpleResponseReturnsTypedErrors(t *testing.T) {
	t.Run("nil command", func(t *testing.T) {
		_, err := simpleResponse(nil, discordgo.EnglishUS)
		if !errors.Is(err, ErrNilCommand) {
			t.Fatalf("expected ErrNilCommand, got %v", err)
		}
	})

	t.Run("nil response template", func(t *testing.T) {
		cmd := &command{Key: Hello}
		_, err := simpleResponse(cmd, discordgo.EnglishUS)
		if !errors.Is(err, ErrNilResponseTemplate) {
			t.Fatalf("expected ErrNilResponseTemplate, got %v", err)
		}
	})

	t.Run("missing default response", func(t *testing.T) {
		withTemporaryDesiredLocale(t, discordgo.PortugueseBR)

		cmd := &command{
			Key: Hello,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.SpanishES: "Hola",
			},
		}

		_, err := simpleResponse(cmd, discordgo.Japanese)
		if !errors.Is(err, ErrMissingDefaultResponseTemplate) {
			t.Fatalf("expected ErrMissingDefaultResponseTemplate, got %v", err)
		}
	})
}
