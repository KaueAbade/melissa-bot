package commands

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func withTemporaryRegistry(t *testing.T, cmds []*command) {
	t.Helper()
	originalCommands := commandsDef
	originalMap := commands

	commandsDef = cmds
	commands = make(map[CommandKey]*command, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			normalizedKey := CommandKey(strings.ToLower(strings.TrimSpace(cmd.Key.String())))
			commands[normalizedKey] = cmd
		}
	}

	t.Cleanup(func() {
		commandsDef = originalCommands
		commands = originalMap
	})
}

func TestGetCmdFromKey(t *testing.T) {
	cmd, exists := getCmdFromKey(CmdHelp)
	if !exists {
		t.Fatalf("expected command for key %q", CmdHelp)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil command for key %q", CmdHelp)
	}

	if _, exists := getCmdFromKey(CommandKey("missing")); exists {
		t.Fatalf("did not expect missing command key to resolve")
	}
}

func TestGetCmdFromName(t *testing.T) {
	cmd, ok := getCmdFromName("  HeLp  ")
	if !ok || cmd == nil {
		t.Fatalf("expected help command to resolve from name")
	}

	if _, ok := getCmdFromName("   "); ok {
		t.Fatalf("did not expect empty command name to resolve")
	}

	if _, ok := getCmdFromName("missing"); ok {
		t.Fatalf("did not expect missing command name to resolve")
	}
}

func TestGetCmdFromContent(t *testing.T) {
	cmd, ok := getCmdFromContent("  PiNg now")
	if !ok || cmd == nil || cmd.Key != CmdPing {
		t.Fatalf("expected ping command from content")
	}

	if _, ok := getCmdFromContent("   "); ok {
		t.Fatalf("did not expect empty content to resolve command")
	}

	if _, ok := getCmdFromContent("unknown args"); ok {
		t.Fatalf("did not expect unknown content to resolve command")
	}
}

func TestGetApplicationCommands(t *testing.T) {
	appCmds := GetApplicationCommands()
	if len(appCmds) != len(commandsDef) {
		t.Fatalf("expected %d application commands, got %d", len(commandsDef), len(appCmds))
	}
	for _, appCmd := range appCmds {
		if appCmd.Name == "" {
			t.Fatalf("definition contains empty name")
		}
		if appCmd.Description == "" {
			t.Fatalf("definition %q has empty description", appCmd.Name)
		}
	}
}

func TestValidateCommandsSuccess(t *testing.T) {
	withTemporaryRegistry(t, []*command{
		{
			Key: CommandKey("ok"),
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "ok",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "ok",
			},
		},
	})

	if err := ValidateCommands(); err != nil {
		t.Fatalf("expected valid commands: %v", err)
	}
}

func TestValidateCommandsFailures(t *testing.T) {
	tests := []struct {
		name string
		cmds []*command
	}{
		{name: "nil command", cmds: []*command{nil}},
		{name: "empty name", cmds: []*command{{
			Key:              "",
			Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
		}}},
		{name: "invalid normalized key", cmds: []*command{{
			Key:              CommandKey("   "),
			Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
		}}},
		{name: "duplicate key", cmds: []*command{
			{
				Key:              CommandKey("dup"),
				Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
				ResponseBuilder:  simpleResponse,
				ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			},
			{
				Key:              CommandKey("dup"),
				Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "y"},
				ResponseBuilder:  simpleResponse,
				ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "y"},
			},
		}},
		{name: "missing response builder", cmds: []*command{{
			Key:              CommandKey("x"),
			Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
		}}},
		{name: "missing descriptions", cmds: []*command{{
			Key:              CommandKey("x"),
			Descriptions:     map[discordgo.Locale]string{},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
		}}},
		{name: "missing default locale description", cmds: []*command{{
			Key:              CommandKey("x"),
			Descriptions:     map[discordgo.Locale]string{discordgo.PortugueseBR: "x"},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
		}}},
		{name: "missing responses", cmds: []*command{{
			Key:              CommandKey("x"),
			Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{},
		}}},
		{name: "missing default locale", cmds: []*command{{
			Key:              CommandKey("x"),
			Descriptions:     map[discordgo.Locale]string{discordgo.EnglishUS: "x"},
			ResponseBuilder:  simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{discordgo.PortugueseBR: "x"},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTemporaryRegistry(t, tt.cmds)
			if err := ValidateCommands(); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestHandleInteractionUnknownCommand(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/interactions/") && strings.HasSuffix(r.URL.Path, "/callback") {
			called = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalEndpointAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/api/v9/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = originalEndpointAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:     "interaction-id",
		Token:  "interaction-token",
		Type:   discordgo.InteractionApplicationCommand,
		Locale: discordgo.EnglishUS,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "missing",
		},
	}}

	HandleInteraction(session, interaction)
	if called {
		t.Fatalf("did not expect interaction callback for unknown command")
	}
}

func TestHandleInteractionSuccess(t *testing.T) {
	called := false
	responseContent := ""
	responseType := discordgo.InteractionResponseType(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/interactions/") && strings.HasSuffix(r.URL.Path, "/callback") {
			called = true
			var payload struct {
				Type int `json:"type"`
				Data struct {
					Content string `json:"content"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode interaction payload: %v", err)
			}
			responseType = discordgo.InteractionResponseType(payload.Type)
			responseContent = payload.Data.Content
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalEndpointAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/api/v9/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = originalEndpointAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:     "interaction-id",
		Token:  "interaction-token",
		Type:   discordgo.InteractionApplicationCommand,
		Locale: discordgo.EnglishUS,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "hello",
		},
	}}

	HandleInteraction(session, interaction)
	if !called {
		t.Fatalf("expected interaction callback request")
	}
	cmd := commands[CmdHello]
	if cmd == nil {
		t.Fatalf("expected command to be registered")
	}
	expected, err := cmd.Response(discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error in command response: %v", err)
	}
	if responseType != discordgo.InteractionResponseChannelMessageWithSource {
		t.Fatalf("expected interaction response type %v, got %v", discordgo.InteractionResponseChannelMessageWithSource, responseType)
	}
	if responseContent != expected {
		t.Fatalf("expected interaction content %q, got %q", expected, responseContent)
	}
}

func TestHandleInteractionRespondError(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/interactions/") && strings.HasSuffix(r.URL.Path, "/callback") {
			called = true
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalEndpointAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/api/v9/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = originalEndpointAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:     "interaction-id",
		Token:  "interaction-token",
		Type:   discordgo.InteractionApplicationCommand,
		Locale: discordgo.EnglishUS,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "hello",
		},
	}}

	HandleInteraction(session, interaction)
	if !called {
		t.Fatalf("expected interaction callback request")
	}
}

func TestHandleInteractionExecuteError(t *testing.T) {
	withTemporaryRegistry(t, []*command{
		{
			Key: CommandKey("broken"),
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "broken",
			},
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "broken",
			},
		},
	})

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/interactions/") && strings.HasSuffix(r.URL.Path, "/callback") {
			called = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	originalEndpointAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/api/v9/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = originalEndpointAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.Client = server.Client()

	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:     "interaction-id",
		Token:  "interaction-token",
		Type:   discordgo.InteractionApplicationCommand,
		Locale: discordgo.EnglishUS,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "broken",
		},
	}}

	HandleInteraction(session, interaction)
	if called {
		t.Fatalf("did not expect interaction callback request when execute fails")
	}
}

func TestExecuteFromKey(t *testing.T) {
	withTemporaryRegistry(t, []*command{
		{
			Key: CmdHello,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "hello",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "hello response",
			},
		},
	})

	got, err := ExecuteFromKey(CmdHello, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello response" {
		t.Fatalf("unexpected response: %q", got)
	}

	if _, err := ExecuteFromKey(CommandKey("missing"), discordgo.EnglishUS); !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestExecuteFromContentAndName(t *testing.T) {
	withTemporaryRegistry(t, []*command{
		{
			Key: CmdPing,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "ping",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "pong",
			},
		},
	})

	got, err := ExecuteFromName("  PING ", discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error from name: %v", err)
	}
	if got != "pong" {
		t.Fatalf("unexpected response from name: %q", got)
	}

	got, err = ExecuteFromContent("  ping now", discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error from content: %v", err)
	}
	if got != "pong" {
		t.Fatalf("unexpected response from content: %q", got)
	}

	if _, err := ExecuteFromContent("   ", discordgo.EnglishUS); !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound for empty content, got %v", err)
	}
}
