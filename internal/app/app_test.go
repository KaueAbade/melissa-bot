package app

import (
	"encoding/json"
	"errors"
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

	originalDiscord := discordgo.EndpointDiscord
	originalAPI := discordgo.EndpointAPI
	originalApplications := discordgo.EndpointApplications
	originalUsers := discordgo.EndpointUsers
	originalGuilds := discordgo.EndpointGuilds
	originalChannels := discordgo.EndpointChannels
	originalGateway := discordgo.EndpointGateway
	originalGatewayBot := discordgo.EndpointGatewayBot

	discordgo.EndpointDiscord = serverURL + "/"
	discordgo.EndpointAPI = serverURL + "/api/v9/"
	discordgo.EndpointApplications = discordgo.EndpointAPI + "applications"
	discordgo.EndpointUsers = serverURL + "/users/"
	discordgo.EndpointGuilds = serverURL + "/guilds/"
	discordgo.EndpointChannels = serverURL + "/channels/"
	discordgo.EndpointGateway = serverURL + "/api/v9/gateway"
	discordgo.EndpointGatewayBot = serverURL + "/api/v9/gateway/bot"

	t.Cleanup(func() {
		discordgo.EndpointDiscord = originalDiscord
		discordgo.EndpointAPI = originalAPI
		discordgo.EndpointApplications = originalApplications
		discordgo.EndpointUsers = originalUsers
		discordgo.EndpointGuilds = originalGuilds
		discordgo.EndpointChannels = originalChannels
		discordgo.EndpointGateway = originalGateway
		discordgo.EndpointGatewayBot = originalGatewayBot
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

func newBareSession(t *testing.T, userID string) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.State.User = &discordgo.User{ID: userID}

	return session
}

func newTestRuntime(commandWipe bool, debug bool) *Runtime {
	runtime := setRuntime(&Config{CommandWipe: commandWipe, Debug: debug})
	runtime.getCommands = commands.GetRegistry().GetApplicationCommands
	return runtime
}

func newDiscordAPIServer(t *testing.T, users map[string]string, guilds map[string]testGuild, sendStatus int) (*httptest.Server, *[]string) {
	t.Helper()

	sentMessages := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v9/gateway":
			_, _ = w.Write([]byte(`{"url":"ws://127.0.0.1:1"}`))
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/"):
			id := strings.TrimPrefix(r.URL.Path, "/users/")
			userLocale := users[id]
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"locale":%q}`, id, userLocale)))
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
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v9/applications/") && strings.HasSuffix(r.URL.Path, "/commands"):
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode command payload: %v", err)
			}
			if payload.Name == "" {
				payload.Name = "unnamed"
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"command-id","name":%q}`, payload.Name)))
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v9/applications/") && strings.Contains(r.URL.Path, "/commands/"):
			w.WriteHeader(http.StatusNoContent)
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
	runtime := newTestRuntime(false, false)

	runtime.MessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "bot-id", Bot: true},
		Content: "hello",
	}})

	runtime.MessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "other-bot", Bot: true},
		Content: "hello",
	}})

	if got := len(*sent); got != 0 {
		t.Fatalf("expected no sent messages, got %d", got)
	}
}

func TestReadyRegistersAllCommands(t *testing.T) {
	runtime := newTestRuntime(false, false)
	session := newBareSession(t, "bot-id")
	session.State.User.Username = "melissa"
	session.State.User.Discriminator = "0001"

	updated := false
	registered := 0
	runtime.updateGameStatus = func(session *discordgo.Session) {
		updated = true
	}
	runtime.registerCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		registered++
		return &discordgo.ApplicationCommand{ID: fmt.Sprintf("id-%d", registered), Name: cmd.Name}, nil
	}

	runtime.Ready(session, &discordgo.Ready{})

	if !updated {
		t.Fatalf("expected game status update")
	}
	if registered != len(commands.GetRegistry().GetApplicationCommands()) {
		t.Fatalf("expected all commands to be registered, got %d", registered)
	}
	if got := len(runtime.registeredCommands); got != registered {
		t.Fatalf("expected %d registered commands in runtime, got %d", registered, got)
	}
}

func TestRunReturnsErrorForNilRuntime(t *testing.T) {
	var runtime *Runtime
	err := runtime.Run()
	if err == nil {
		t.Fatalf("expected runtime error")
	}
}

func TestRunReturnsOpenError(t *testing.T) {
	runtime := newTestRuntime(false, false)
	runtime.session = newBareSession(t, "bot-id")

	openErr := errors.New("open failed")
	runtime.openSession = func(session *discordgo.Session) error {
		return openErr
	}

	err := runtime.Run()
	if !errors.Is(err, openErr) {
		t.Fatalf("expected open error, got %v", err)
	}
}

func TestShutdownWipesCommandsWhenEnabled(t *testing.T) {
	runtime := newTestRuntime(true, false)
	runtime.session = newBareSession(t, "bot-id")
	runtime.registeredCommands = []*discordgo.ApplicationCommand{
		{ID: "c1", Name: "one"},
		{ID: "c2", Name: "two"},
	}

	deleted := 0
	runtime.deleteCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) error {
		deleted++
		return nil
	}
	runtime.closeSession = func(session *discordgo.Session) error { return nil }

	runtime.Shutdown()

	if deleted != 2 {
		t.Fatalf("expected two deletions, got %d", deleted)
	}
}

func TestRespondToMessageUsesFallbackWhenCommandNotFound(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	runtime := newTestRuntime(false, false)
	session := newTestSession(t, server.Client())

	runtime.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "unknown",
	}})

	if got := len(*sent); got > 1 {
		t.Fatalf("expected at most one sent message, got %d", got)
	}
}

func TestGetRuntime(t *testing.T) {
	original := appRuntimeInstance
	t.Cleanup(func() {
		appRuntimeInstance = original
	})

	appRuntimeInstance = nil
	if got := GetRuntime(); got != nil {
		t.Fatalf("expected nil runtime, got %#v", got)
	}

	runtime := setRuntime(&Config{})
	if got := GetRuntime(); got != runtime {
		t.Fatalf("expected shared runtime instance")
	}
}

func TestSetupSessionInitializesRuntime(t *testing.T) {
	originalNewSession := newSession
	originalFatal := setupSessionFatal
	t.Cleanup(func() {
		newSession = originalNewSession
		setupSessionFatal = originalFatal
	})

	setupSessionFatal = func(v ...interface{}) {
		t.Fatalf("did not expect setup fatal call: %v", v)
	}

	SetupSession(&Config{DiscordToken: "token", CommandWipe: true, Debug: true})

	runtime := GetRuntime()
	if runtime == nil || runtime.session == nil {
		t.Fatalf("expected runtime session to be initialized")
	}

	if !runtime.commandWipe || !runtime.debug {
		t.Fatalf("expected runtime config flags to be set")
	}

	expectedIntents := discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	if runtime.session.Identify.Intents != expectedIntents {
		t.Fatalf("unexpected intents: got %d want %d", runtime.session.Identify.Intents, expectedIntents)
	}
}

func TestSetupSessionHandlesNewSessionError(t *testing.T) {
	originalNewSession := newSession
	originalFatal := setupSessionFatal
	t.Cleanup(func() {
		newSession = originalNewSession
		setupSessionFatal = originalFatal
	})

	newSession = func(token string) (*discordgo.Session, error) {
		return nil, errors.New("session init failed")
	}
	setupSessionFatal = func(v ...interface{}) {
		panic(v)
	}

	defer func() {
		if recover() == nil {
			t.Fatalf("expected setup fatal panic")
		}
	}()

	SetupSession(&Config{DiscordToken: "token"})
}

func TestSetRuntimeDefaultHooks(t *testing.T) {
	server, _ := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	runtime := setRuntime(&Config{})
	session := newTestSession(t, server.Client())

	runtime.updateGameStatus(session)

	cmd, err := runtime.registerCommand(session, &discordgo.ApplicationCommand{Name: "ping"})
	if err != nil {
		t.Fatalf("expected register command to succeed: %v", err)
	}

	if err := runtime.deleteCommand(session, cmd); err != nil {
		t.Fatalf("expected delete command to succeed: %v", err)
	}

	if err := runtime.closeSession(session); err != nil {
		t.Fatalf("expected close session to succeed: %v", err)
	}

	if err := runtime.openSession(session); err == nil {
		t.Fatalf("expected open session to fail without websocket server")
	}
}

func TestRunSuccess(t *testing.T) {
	runtime := newTestRuntime(false, false)
	runtime.session = newBareSession(t, "bot-id")
	runtime.openSession = func(session *discordgo.Session) error {
		return nil
	}

	if err := runtime.Run(); err != nil {
		t.Fatalf("expected run success, got %v", err)
	}
}

func TestReadyPanicsWhenRegisterCommandFails(t *testing.T) {
	runtime := newTestRuntime(false, true)
	runtime.getCommands = func() []*discordgo.ApplicationCommand {
		return []*discordgo.ApplicationCommand{{Name: "help"}}
	}
	runtime.registerCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		return nil, errors.New("register failed")
	}
	runtime.updateGameStatus = func(session *discordgo.Session) {}

	session := newBareSession(t, "bot-id")
	session.State.User.Username = "melissa"
	session.State.User.Discriminator = "0001"

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic when register command fails")
		}
	}()

	runtime.Ready(session, &discordgo.Ready{})
}

func TestReadyDebugRegistersCommands(t *testing.T) {
	runtime := newTestRuntime(false, true)
	runtime.getCommands = func() []*discordgo.ApplicationCommand {
		return []*discordgo.ApplicationCommand{{Name: "help"}}
	}
	runtime.updateGameStatus = func(session *discordgo.Session) {}
	runtime.registerCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		return &discordgo.ApplicationCommand{ID: "id-1", Name: cmd.Name}, nil
	}

	session := newBareSession(t, "bot-id")
	session.State.User.Username = "melissa"
	session.State.User.Discriminator = "0001"

	runtime.Ready(session, &discordgo.Ready{})

	if len(runtime.registeredCommands) != 1 {
		t.Fatalf("expected one registered command, got %d", len(runtime.registeredCommands))
	}
}

func TestInteractionCreateDelegatesToCommandRegistry(t *testing.T) {
	runtime := newTestRuntime(false, false)
	session := newBareSession(t, "bot-id")

	runtime.InteractionCreate(session, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "unknown"},
	}})
}

func TestShutdownEdgeCases(t *testing.T) {
	var runtime *Runtime
	runtime.Shutdown()

	runtime = newTestRuntime(false, false)
	runtime.session = nil
	runtime.Shutdown()

	runtime = newTestRuntime(false, false)
	runtime.session = newBareSession(t, "bot-id")
	closed := false
	runtime.closeSession = func(session *discordgo.Session) error {
		closed = true
		return errors.New("close failed")
	}
	runtime.Shutdown()
	if !closed {
		t.Fatalf("expected session close to be attempted")
	}

	runtime = newTestRuntime(true, false)
	runtime.session = newBareSession(t, "bot-id")
	runtime.registeredCommands = []*discordgo.ApplicationCommand{{ID: "c1", Name: "one"}}
	deleted := 0
	runtime.deleteCommand = func(session *discordgo.Session, cmd *discordgo.ApplicationCommand) error {
		deleted++
		return errors.New("delete failed")
	}
	runtime.closeSession = func(session *discordgo.Session) error { return nil }
	runtime.Shutdown()
	if deleted != 1 {
		t.Fatalf("expected one deletion attempt, got %d", deleted)
	}
}

func TestMessageCreateRoutesAndTrimMention(t *testing.T) {
	server, sent := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusOK)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	runtime := newTestRuntime(false, true)
	session := newTestSession(t, server.Client())

	mentionMessage := &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1", Username: "u", Discriminator: "0001"},
		Mentions:  []*discordgo.User{{ID: "bot-id"}},
		Content:   "<@bot-id> help",
	}}
	runtime.MessageCreate(session, mentionMessage)
	if mentionMessage.Content != "help" {
		t.Fatalf("expected mention prefix to be trimmed, got %q", mentionMessage.Content)
	}

	altMentionMessage := &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-2", Username: "u", Discriminator: "0002"},
		Mentions:  []*discordgo.User{{ID: "bot-id"}},
		Content:   "<@!bot-id> help",
	}}
	runtime.MessageCreate(session, altMentionMessage)
	if altMentionMessage.Content != "help" {
		t.Fatalf("expected alternate mention prefix to be trimmed, got %q", altMentionMessage.Content)
	}

	beforeDirect := len(*sent)
	runtime.MessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-3", Username: "u", Discriminator: "0003"},
		Content:   "help",
	}})
	if len(*sent) <= beforeDirect {
		t.Fatalf("expected direct message route to send a response")
	}

	beforeGuild := len(*sent)
	runtime.MessageCreate(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		GuildID:   "guild-1",
		Author:    &discordgo.User{ID: "user-4", Username: "u", Discriminator: "0004"},
		Content:   "help",
	}})
	if len(*sent) != beforeGuild {
		t.Fatalf("expected guild route to ignore message")
	}
}

func TestRespondToMessageSendError(t *testing.T) {
	server, _ := newDiscordAPIServer(t, map[string]string{}, map[string]testGuild{}, http.StatusInternalServerError)
	defer server.Close()
	setupDiscordEndpoints(t, server.URL)

	runtime := newTestRuntime(false, false)
	session := newTestSession(t, server.Client())

	runtime.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "help",
	}})
}

func TestRespondToMessageFallbackFailure(t *testing.T) {
	runtime := newTestRuntime(false, false)
	runtime.executeFromContent = func(string, discordgo.Locale) (string, error) {
		return "", errors.New("execute failed")
	}
	runtime.executeFallback = func(discordgo.Locale) (string, error) {
		return "", errors.New("fallback failed")
	}

	session := newBareSession(t, "bot-id")
	runtime.respondToMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "channel-1",
		GuildID:   "guild-1",
		Author:    &discordgo.User{ID: "user-1"},
		Content:   "unknown",
	}})
}
