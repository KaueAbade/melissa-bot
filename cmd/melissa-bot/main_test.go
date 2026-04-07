package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KaueAbade/melissa-bot/internal/commands"
	"github.com/bwmarrin/discordgo"
)

type testGuild struct {
	ownerID         string
	preferredLocale string
}

func setupDiscordEndpoints(t *testing.T, serverURL string) {
	t.Helper()

	originalUsers := discordgo.EndpointUsers
	originalGuilds := discordgo.EndpointGuilds
	originalChannels := discordgo.EndpointChannels

	discordgo.EndpointUsers = serverURL + "/users/"
	discordgo.EndpointGuilds = serverURL + "/guilds/"
	discordgo.EndpointChannels = serverURL + "/channels/"

	t.Cleanup(func() {
		discordgo.EndpointUsers = originalUsers
		discordgo.EndpointGuilds = originalGuilds
		discordgo.EndpointChannels = originalChannels
	})
}

func newTestSession(t *testing.T, client *http.Client) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = client
	session.State.User = &discordgo.User{ID: "bot-id"}

	return session
}

func newDiscordAPIServer(t *testing.T, users map[string]string, guilds map[string]testGuild, sendStatus int) (*httptest.Server, *[]string) {
	t.Helper()

	sentMessages := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/"):
			id := strings.TrimPrefix(r.URL.Path, "/users/")
			locale := users[id]
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"locale":%q}`, id, locale)))
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/"):
			id := strings.TrimPrefix(r.URL.Path, "/guilds/")
			guild, ok := guilds[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"owner_id":%q,"preferred_locale":%q}`,
				id, guild.ownerID, guild.preferredLocale)))
			return
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/channels/") && strings.HasSuffix(r.URL.Path, "/messages"):
			if sendStatus >= 400 {
				w.WriteHeader(sendStatus)
				_, _ = w.Write([]byte(`{"message":"send failed"}`))
				return
			}

			var payload struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode channel message payload: %v", err)
			}
			sentMessages = append(sentMessages, payload.Content)

			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			channelID := ""
			if len(parts) >= 2 {
				channelID = parts[1]
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"message-id","channel_id":%q,"content":%q}`,
				channelID, payload.Content)))
			return
		default:
			http.NotFound(w, r)
		}
	}))

	return server, &sentMessages
}

func TestMessageCreateIgnoresBotMessages(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "bot-id", Bot: true},
		Content: "hello",
	}})

	messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "other-bot", Bot: true},
		Content: "hello",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no sent messages, got %d", got)
	}
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

	locale := resolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author: &discordgo.User{ID: "user-dm"},
	}})
	if locale != discordgo.PortugueseBR {
		t.Fatalf("expected pt-BR locale, got %s", locale)
	}
}

func TestResolveMessageLocaleGuildOwnerLocale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/guilds/guild-1":
			_, _ = w.Write([]byte(`{"id":"guild-1","owner_id":"owner-1","preferred_locale":"en-US"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/owner-1":
			_, _ = w.Write([]byte(`{"id":"owner-1","locale":"pt-BR"}`))
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

	locale := resolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1",
		Author:  &discordgo.User{ID: "author"},
	}})
	if locale != discordgo.PortugueseBR {
		t.Fatalf("expected owner locale pt-BR, got %s", locale)
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

	locale := resolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-2",
		Author:  &discordgo.User{ID: "author"},
	}})
	if locale != discordgo.Locale("de") {
		t.Fatalf("expected guild preferred locale de, got %s", locale)
	}
}

func TestResolveMessageLocaleDefaultFallback(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
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

	locale := resolveMessageLocale(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "unknown",
		Author:  &discordgo.User{ID: "author"},
	}})
	if locale != discordgo.EnglishUS {
		t.Fatalf("expected default locale en-US, got %s", locale)
	}
}

func TestGuildMessageCreateNoop(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1",
		Author:  &discordgo.User{ID: "author"},
		Content: "ping",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no sent messages for guild route, got %d", got)
	}
}

func TestMentionMessageCreateExecutesCommand(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"user-1": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Help, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute help: %v", err)
	}

	mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "<@bot-id> help",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected help response %q, got %q", expected, got)
	}
}

func TestMentionMessageCreateSupportsAltMentionAndFallback(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"user-1": string(discordgo.PortugueseBR)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Hello, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute fallback hello: %v", err)
	}

	mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "<@!bot-id> unknown",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected fallback response %q, got %q", expected, got)
	}
}

func TestDirectMessageCreateExecutesCommand(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"dm-user": string(discordgo.PortugueseBR)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Ping, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute ping: %v", err)
	}

	directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "dm-user"},
		Content:   "PING",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected ping response %q, got %q", expected, got)
	}
}

func TestDirectMessageCreateFallback(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"dm-user": string(discordgo.PortugueseBR)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Hello, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute fallback hello: %v", err)
	}

	directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "dm-user"},
		Content:   "unknown",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected fallback response %q, got %q", expected, got)
	}
}

func TestMentionMessageCreateSendErrorDoesNotPanic(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"user-1": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusInternalServerError,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "<@bot-id> help",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no captured sent messages on send error, got %d", got)
	}
}

func TestDirectMessageCreateSendErrorDoesNotPanic(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"dm-user": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusInternalServerError,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "dm-user"},
		Content:   "help",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no captured sent messages on send error, got %d", got)
	}
}

func TestEntrypointsParityForDeterministicCommands(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"user-1": string(discordgo.EnglishUS), "dm-user": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	deterministicCommands := []commands.CommandKey{commands.Help, commands.Hello, commands.Ping}
	for _, key := range deterministicCommands {
		*sent = (*sent)[:0]

		expected, err := commands.ExecuteFromKey(key, discordgo.EnglishUS)
		if err != nil {
			t.Fatalf("execute expected response for %s: %v", key, err)
		}

		mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
			ChannelID: "channel-1",
			Author:    &discordgo.User{ID: "user-1"},
			Content:   "<@bot-id> " + key.String(),
		}})
		if got := len(*sent); got != 1 {
			t.Fatalf("expected one mention response for %s, got %d", key, got)
		}
		mentionResponse := (*sent)[0]

		*sent = (*sent)[:0]
		directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
			ChannelID: "channel-1",
			Author:    &discordgo.User{ID: "dm-user"},
			Content:   key.String(),
		}})
		if got := len(*sent); got != 1 {
			t.Fatalf("expected one dm response for %s, got %d", key, got)
		}
		dmResponse := (*sent)[0]

		if mentionResponse != expected {
			t.Fatalf("expected mention parity for %s: expected %q, got %q", key, expected, mentionResponse)
		}
		if dmResponse != expected {
			t.Fatalf("expected dm parity for %s: expected %q, got %q", key, expected, dmResponse)
		}
		if mentionResponse != dmResponse {
			t.Fatalf("expected mention and dm parity for %s, got mention=%q dm=%q", key, mentionResponse, dmResponse)
		}
	}
}

func TestMessageCreateRoutesMentionBeforeGuild(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Ping, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute ping: %v", err)
	}

	messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Mentions:  []*discordgo.User{{ID: "other"}, {ID: "bot-id"}},
		Content:   "<@bot-id> ping",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected mention route response %q, got %q", expected, got)
	}
}

func TestMessageCreateRoutesDirectMessage(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"dm-user": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	session := newTestSession(t, server.Client())

	expected, err := commands.ExecuteFromKey(commands.Help, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute help: %v", err)
	}

	messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "dm-user"},
		Content:   "help",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != expected {
		t.Fatalf("expected dm route response %q, got %q", expected, got)
	}
}
