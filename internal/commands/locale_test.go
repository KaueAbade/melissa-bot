package commands

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestResolveLocalizedText(t *testing.T) {
	withTemporaryDesiredLocale(t, discordgo.PortugueseBR)

	t.Run("requested locale", func(t *testing.T) {
		texts := map[discordgo.Locale]string{
			discordgo.EnglishUS: "english",
			discordgo.Japanese:  "japanese",
		}

		got, ok := resolveLocalizedText(texts, discordgo.Japanese)
		if !ok {
			t.Fatalf("expected requested locale to resolve")
		}
		if got != "japanese" {
			t.Fatalf("expected japanese text, got %q", got)
		}
	})

	t.Run("desired locale fallback", func(t *testing.T) {
		texts := map[discordgo.Locale]string{
			discordgo.EnglishUS:    "english",
			discordgo.PortugueseBR: "portuguese",
		}

		got, ok := resolveLocalizedText(texts, discordgo.SpanishES)
		if !ok {
			t.Fatalf("expected desired locale fallback to resolve")
		}
		if got != "portuguese" {
			t.Fatalf("expected portuguese text, got %q", got)
		}
	})

	t.Run("default locale fallback", func(t *testing.T) {
		texts := map[discordgo.Locale]string{
			discordgo.EnglishUS: "english",
		}

		got, ok := resolveLocalizedText(texts, discordgo.SpanishES)
		if !ok {
			t.Fatalf("expected default locale fallback to resolve")
		}
		if got != "english" {
			t.Fatalf("expected english text, got %q", got)
		}
	})

	t.Run("no fallback match", func(t *testing.T) {
		texts := map[discordgo.Locale]string{
			discordgo.SpanishES: "espanol",
		}

		got, ok := resolveLocalizedText(texts, discordgo.Japanese)
		if ok {
			t.Fatalf("expected unresolved locale, got %q", got)
		}
	})

	t.Run("nil map", func(t *testing.T) {
		got, ok := resolveLocalizedText(nil, discordgo.Japanese)
		if ok {
			t.Fatalf("expected nil map to be unresolved, got %q", got)
		}
	})
}
