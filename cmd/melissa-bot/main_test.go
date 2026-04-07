package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func newTestRuntime() *appRuntime {
	return newAppRuntime(false, false, commands.GetRegistry())
}

func newBareSession(t *testing.T, userID string) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.State.User = &discordgo.User{ID: userID}

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
	app := newTestRuntime()

	app.messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "bot-id", Bot: true},
		Content: "hello",
	}})

	app.messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	app.messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Help, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute help: %v", err)
	}

	app.mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Hello, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute fallback hello: %v", err)
	}

	app.mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Ping, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute ping: %v", err)
	}

	app.directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Hello, discordgo.PortugueseBR)
	if err != nil {
		t.Fatalf("execute fallback hello: %v", err)
	}

	app.directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	app.mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	app.directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	deterministicCommands := []commands.CommandKey{commands.Help, commands.Hello, commands.Ping}
	for _, key := range deterministicCommands {
		*sent = (*sent)[:0]

		expected, err := commands.GetRegistry().ExecuteFromKey(key, discordgo.EnglishUS)
		if err != nil {
			t.Fatalf("execute expected response for %s: %v", key, err)
		}

		app.mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
			ChannelID: "channel-1",
			Author:    &discordgo.User{ID: "user-1"},
			Content:   "<@bot-id> " + key.String(),
		}})
		if got := len(*sent); got != 1 {
			t.Fatalf("expected one mention response for %s, got %d", key, got)
		}
		mentionResponse := (*sent)[0]

		*sent = (*sent)[:0]
		app.directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Ping, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute ping: %v", err)
	}

	app.messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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
	app := newTestRuntime()

	expected, err := commands.GetRegistry().ExecuteFromKey(commands.Help, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("execute help: %v", err)
	}

	app.messageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
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

func TestRunReturnsErrorForNilRuntime(t *testing.T) {
	err := run(nil, func(c chan<- os.Signal, sig ...os.Signal) {})
	if err == nil {
		t.Fatalf("expected runtime error")
	}
}

func TestRunOpensAndShutsDown(t *testing.T) {
	app := newTestRuntime()
	app.discord = newBareSession(t, "bot-id")

	opened := false
	closed := false
	app.openSession = func(session *discordgo.Session) error {
		opened = true
		return nil
	}
	app.closeSession = func(session *discordgo.Session) error {
		closed = true
		return nil
	}

	err := run(app, func(c chan<- os.Signal, sig ...os.Signal) {
		c <- os.Interrupt
	})
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
	if !opened {
		t.Fatalf("expected openSession to be called")
	}
	if !closed {
		t.Fatalf("expected closeSession to be called")
	}
}

func TestRunReturnsOpenError(t *testing.T) {
	app := newTestRuntime()
	app.discord = newBareSession(t, "bot-id")

	openErr := errors.New("open failed")
	app.openSession = func(session *discordgo.Session) error {
		return openErr
	}

	err := run(app, func(c chan<- os.Signal, sig ...os.Signal) {})
	if !errors.Is(err, openErr) {
		t.Fatalf("expected open error, got %v", err)
	}
}

func TestShutdownWipesCommandsWhenEnabled(t *testing.T) {
	app := newAppRuntime(true, false, commands.GetRegistry())
	app.discord = newBareSession(t, "bot-id")
	app.registeredCommands = []*discordgo.ApplicationCommand{
		{ID: "c1", Name: "one"},
		{ID: "c2", Name: "two"},
	}

	deleted := 0
	app.deleteCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) error {
		deleted++
		return nil
	}
	app.closeSession = func(session *discordgo.Session) error { return nil }

	app.shutdown()

	if deleted != 2 {
		t.Fatalf("expected two deletions, got %d", deleted)
	}
}

func TestShutdownNoopForNilRuntime(t *testing.T) {
	var app *appRuntime
	app.shutdown()
}

func TestShutdownSkipsWipeWhenDisabled(t *testing.T) {
	app := newAppRuntime(false, false, commands.GetRegistry())
	app.discord = newBareSession(t, "bot-id")

	deleted := 0
	closed := false
	app.deleteCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) error {
		deleted++
		return nil
	}
	app.closeSession = func(session *discordgo.Session) error {
		closed = true
		return nil
	}

	app.shutdown()

	if deleted != 0 {
		t.Fatalf("expected no deletions, got %d", deleted)
	}
	if !closed {
		t.Fatalf("expected closeSession to be called")
	}
}

func TestReadyPanicsWhenCommandRegistrationFails(t *testing.T) {
	app := newTestRuntime()
	session := newBareSession(t, "bot-id")
	app.getCommands = func() []*discordgo.ApplicationCommand {
		return []*discordgo.ApplicationCommand{{Name: "help", Description: "help"}}
	}
	app.registerCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		return nil, errors.New("register failed")
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected panic when command registration fails")
		}
	}()

	app.ready(session, &discordgo.Ready{})
}

func TestReadyRegistersAllCommands(t *testing.T) {
	app := newTestRuntime()
	session := newBareSession(t, "bot-id")
	session.State.User.Username = "melissa"
	session.State.User.Discriminator = "0001"

	updated := false
	registered := 0
	app.updateGameStatus = func(session *discordgo.Session) {
		updated = true
	}
	app.registerCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		registered++
		return &discordgo.ApplicationCommand{ID: fmt.Sprintf("id-%d", registered), Name: cmd.Name}, nil
	}

	app.ready(session, &discordgo.Ready{})

	if !updated {
		t.Fatalf("expected game status update")
	}
	if registered != len(commands.GetRegistry().GetApplicationCommands()) {
		t.Fatalf("expected all commands to be registered, got %d", registered)
	}
	if got := len(app.registeredCommands); got != registered {
		t.Fatalf("expected %d registered commands in runtime, got %d", registered, got)
	}
}

func TestRespondToMessageUsesFallbackWhenCommandNotFound(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	app := newTestRuntime()
	session := newTestSession(t, server.Client())

	app.execFromContent = func(content string, locale discordgo.Locale) (string, error) {
		return "", commands.ErrCommandNotFound
	}
	app.execFromKey = func(key commands.CommandKey, locale discordgo.Locale) (string, error) {
		return "fallback", nil
	}

	app.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "unknown",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != "fallback" {
		t.Fatalf("expected fallback response, got %q", got)
	}
}

func TestRespondToMessageReturnsWhenFallbackFails(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	app := newTestRuntime()
	session := newTestSession(t, server.Client())

	app.execFromContent = func(content string, locale discordgo.Locale) (string, error) {
		return "", commands.ErrCommandNotFound
	}
	app.execFromKey = func(key commands.CommandKey, locale discordgo.Locale) (string, error) {
		return "", errors.New("fallback failed")
	}

	app.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "unknown",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no message sent when fallback fails, got %d", got)
	}
}

func TestRespondToMessageLogsNonCommandNotFoundAndFallsBack(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	app := newTestRuntime()
	session := newTestSession(t, server.Client())

	app.execFromContent = func(content string, locale discordgo.Locale) (string, error) {
		return "", errors.New("unexpected")
	}
	app.execFromKey = func(key commands.CommandKey, locale discordgo.Locale) (string, error) {
		return "hello", nil
	}

	app.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "unknown",
	}})

	if got := len(*sent); got != 1 {
		t.Fatalf("expected one sent message, got %d", got)
	}
	if got := (*sent)[0]; got != "hello" {
		t.Fatalf("expected fallback hello response, got %q", got)
	}
}

func TestDebugBranchesInMessageHandlers(t *testing.T) {
	server, sent := newDiscordAPIServer(t,
		map[string]string{"user-1": string(discordgo.EnglishUS), "dm-user": string(discordgo.EnglishUS)},
		map[string]testGuild{},
		http.StatusOK,
	)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	app := newAppRuntime(false, true, commands.GetRegistry())
	session := newTestSession(t, server.Client())

	app.guildMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1",
		Author:  &discordgo.User{ID: "user-1"},
		Content: "hello",
	}})

	app.mentionMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "<@bot-id> help",
	}})

	app.directMessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "dm-user"},
		Content:   "help",
	}})

	if got := len(*sent); got != 2 {
		t.Fatalf("expected two sent messages from mention and dm handlers, got %d", got)
	}
}

func TestNewAppRuntimeDefaultCommandOps(t *testing.T) {
	app := newAppRuntime(false, false, commands.GetRegistry())
	session := newTestSession(t, http.DefaultClient)

	if err := app.closeSession(session); err != nil {
		t.Fatalf("expected closeSession nil error, got %v", err)
	}

	originalGlobalCommands := discordgo.EndpointApplicationGlobalCommands
	originalGlobalCommand := discordgo.EndpointApplicationGlobalCommand
	t.Cleanup(func() {
		discordgo.EndpointApplicationGlobalCommands = originalGlobalCommands
		discordgo.EndpointApplicationGlobalCommand = originalGlobalCommand
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/applications/bot-id/commands":
			_, _ = w.Write([]byte(`{"id":"cmd-1","name":"help","description":"help"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/applications/bot-id/commands/cmd-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	discordgo.EndpointApplicationGlobalCommands = func(appID string) string {
		return server.URL + "/applications/" + appID + "/commands"
	}
	discordgo.EndpointApplicationGlobalCommand = func(appID, cmdID string) string {
		return server.URL + "/applications/" + appID + "/commands/" + cmdID
	}
	session.Client = server.Client()

	created, err := app.registerCommand(session, &discordgo.ApplicationCommand{Name: "help", Description: "help"})
	if err != nil {
		t.Fatalf("register command failed: %v", err)
	}
	if created == nil || created.ID != "cmd-1" {
		t.Fatalf("unexpected created command: %#v", created)
	}

	if err := app.deleteCommand(session, &discordgo.ApplicationCommand{ID: "cmd-1", Name: "help"}); err != nil {
		t.Fatalf("delete command failed: %v", err)
	}
}
