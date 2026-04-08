package locale

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestResolveLocalizedText(t *testing.T) {
	registry := RegistryConfig{
		DesiredLocale: discordgo.PortugueseBR,
		DefaultLocale: discordgo.EnglishUS,
	}

	t.Run("requested locale", func(t *testing.T) {
		texts := map[discordgo.Locale]string{
			discordgo.EnglishUS: "english",
			discordgo.Japanese:  "japanese",
		}

		got, ok := registry.ResolveLocalizedText(texts, discordgo.Japanese)
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

		got, ok := registry.ResolveLocalizedText(texts, discordgo.SpanishES)
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

		got, ok := registry.ResolveLocalizedText(texts, discordgo.SpanishES)
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

		got, ok := registry.ResolveLocalizedText(texts, discordgo.Japanese)
		if ok {
			t.Fatalf("expected unresolved locale, got %q", got)
		}
	})

	t.Run("nil map", func(t *testing.T) {
		got, ok := registry.ResolveLocalizedText(nil, discordgo.Japanese)
		if ok {
			t.Fatalf("expected nil map to be unresolved, got %q", got)
		}
	})
}

func TestResolveMessageLocaleDMUserLocale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/users/user-dm" {
			_, _ = w.Write([]byte(`{"id":"user-dm","locale":"pt-BR"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalUsers := discordgo.EndpointUsers
	discordgo.EndpointUsers = server.URL + "/users/"
	t.Cleanup(func() {
		discordgo.EndpointUsers = originalUsers
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	got := ResolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author: &discordgo.User{ID: "user-dm"},
	}})
	if got != discordgo.PortugueseBR {
		t.Fatalf("expected pt-BR locale, got %s", got)
	}
}

func TestResolveMessageLocaleGuildPreferredLocaleFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/guilds/guild-2":
			_, _ = w.Write([]byte(`{"id":"guild-2","owner_id":"owner-2","preferred_locale":"de"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/owner-2":
			_, _ = w.Write([]byte(`{"id":"owner-2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalUsers := discordgo.EndpointUsers
	originalGuilds := discordgo.EndpointGuilds
	discordgo.EndpointUsers = server.URL + "/users/"
	discordgo.EndpointGuilds = server.URL + "/guilds/"
	t.Cleanup(func() {
		discordgo.EndpointUsers = originalUsers
		discordgo.EndpointGuilds = originalGuilds
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	got := ResolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-2",
		Author:  &discordgo.User{ID: "author"},
	}})
	if got != discordgo.Locale("de") {
		t.Fatalf("expected guild preferred locale de, got %s", got)
	}
}

func TestResolveMessageLocaleGuildOwnerLocale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/guilds/guild-1":
			_, _ = w.Write([]byte(`{"id":"guild-1","owner_id":"owner-1","preferred_locale":"de"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/owner-1":
			_, _ = w.Write([]byte(`{"id":"owner-1","locale":"ja"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalUsers := discordgo.EndpointUsers
	originalGuilds := discordgo.EndpointGuilds
	discordgo.EndpointUsers = server.URL + "/users/"
	discordgo.EndpointGuilds = server.URL + "/guilds/"
	t.Cleanup(func() {
		discordgo.EndpointUsers = originalUsers
		discordgo.EndpointGuilds = originalGuilds
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	got := ResolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1",
		Author:  &discordgo.User{ID: "author"},
	}})
	if got != discordgo.Japanese {
		t.Fatalf("expected guild owner locale ja, got %s", got)
	}
}

func TestResolveMessageLocaleDefaultsToEnglishUS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalUsers := discordgo.EndpointUsers
	originalGuilds := discordgo.EndpointGuilds
	discordgo.EndpointUsers = server.URL + "/users/"
	discordgo.EndpointGuilds = server.URL + "/guilds/"
	t.Cleanup(func() {
		discordgo.EndpointUsers = originalUsers
		discordgo.EndpointGuilds = originalGuilds
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	gotDM := ResolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author: &discordgo.User{ID: "missing-user"},
	}})
	if gotDM != discordgo.EnglishUS {
		t.Fatalf("expected english fallback for DM, got %s", gotDM)
	}

	gotGuild := ResolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "missing-guild",
		Author:  &discordgo.User{ID: "author"},
	}})
	if gotGuild != discordgo.EnglishUS {
		t.Fatalf("expected english fallback for guild message, got %s", gotGuild)
	}
}
