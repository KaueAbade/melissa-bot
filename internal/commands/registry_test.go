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

func withTemporaryRegistry(t *testing.T, cmds []*command) *Registry {
	t.Helper()
	originalRegistry := commandRegistryInstance
	desiredLocale := discordgo.EnglishUS
	if originalRegistry != nil {
		desiredLocale = originalRegistry.GetDesiredLocale()
	}
	testRegistry := newRegistry(cmds, desiredLocale)
	commandRegistryInstance = testRegistry

	t.Cleanup(func() {
		commandRegistryInstance = originalRegistry
	})

	return testRegistry
}

func withTemporaryDesiredLocale(t *testing.T, locale discordgo.Locale) {
	t.Helper()
	registry := GetRegistry()
	originalLocale := registry.GetDesiredLocale()
	registry.SetDesiredLocale(locale)

	t.Cleanup(func() {
		registry.SetDesiredLocale(originalLocale)
	})
}

func TestSetDesiredLocale(t *testing.T) {
	registry := GetRegistry()
	originalLocale := registry.GetDesiredLocale()
	t.Cleanup(func() { registry.SetDesiredLocale(originalLocale) })

	registry.SetDesiredLocale(discordgo.PortugueseBR)

	if got := registry.GetDesiredLocale(); got != discordgo.PortugueseBR {
		t.Fatalf("expected desired locale to be %s, got %s", discordgo.PortugueseBR, got)
	}
}

func TestSetDesiredLocaleAffectsResponseFallback(t *testing.T) {
	withTemporaryDesiredLocale(t, discordgo.EnglishUS)

	cmd := &command{
		Key:             Hello,
		ResponseBuilder: simpleResponse,
		ResponseTemplate: map[discordgo.Locale]string{
			discordgo.EnglishUS:    "Hello",
			discordgo.PortugueseBR: "Ola",
		},
	}

	GetRegistry().SetDesiredLocale(discordgo.PortugueseBR)

	got, err := simpleResponse(cmd, discordgo.Japanese)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Ola" {
		t.Fatalf("expected Portuguese fallback response, got %q", got)
	}
}

func TestGetRegistryCreatesNewWhenNil(t *testing.T) {
	originalRegistry := commandRegistryInstance
	t.Cleanup(func() {
		commandRegistryInstance = originalRegistry
	})

	commandRegistryInstance = nil
	registry := GetRegistry()

	if registry == nil {
		t.Fatalf("expected GetRegistry() to create a new registry when commandRegistryInstance is nil")
	}
	if len(registry.commands) != 0 {
		t.Fatalf("expected newly created registry to have no commands, got %d", len(registry.commands))
	}
	if registry.GetDesiredLocale() != discordgo.EnglishUS {
		t.Fatalf("expected new registry to have default locale, got %s", registry.GetDesiredLocale())
	}
}

func TestGetCmdFromKey(t *testing.T) {
	registry := GetRegistry()
	cmd, exists := registry.getCmdFromKey(Help)
	if !exists {
		t.Fatalf("expected command for key %q", Help)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil command for key %q", Help)
	}

	if _, exists := registry.getCmdFromKey(CommandKey("missing")); exists {
		t.Fatalf("did not expect missing command key to resolve")
	}
}

func TestGetCmdFromName(t *testing.T) {
	registry := GetRegistry()
	cmd, ok := registry.getCmdFromName("  HeLp  ")
	if !ok || cmd == nil {
		t.Fatalf("expected help command to resolve from name")
	}

	if _, ok := registry.getCmdFromName("   "); ok {
		t.Fatalf("did not expect empty command name to resolve")
	}

	if _, ok := registry.getCmdFromName("missing"); ok {
		t.Fatalf("did not expect missing command name to resolve")
	}
}

func TestGetCmdFromContent(t *testing.T) {
	registry := GetRegistry()
	cmd, ok := registry.getCmdFromContent("  PiNg now")
	if !ok || cmd == nil || cmd.Key != Ping {
		t.Fatalf("expected ping command from content")
	}

	if _, ok := registry.getCmdFromContent("   "); ok {
		t.Fatalf("did not expect empty content to resolve command")
	}

	if _, ok := registry.getCmdFromContent("unknown args"); ok {
		t.Fatalf("did not expect unknown content to resolve command")
	}
}

func TestGetApplicationCommands(t *testing.T) {
	registry := GetRegistry()
	appCmds := registry.GetApplicationCommands()
	defs := registry.getCommandDefinitionsSnapshot()
	if len(appCmds) != len(defs) {
		t.Fatalf("expected %d application commands, got %d", len(defs), len(appCmds))
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
	registry := withTemporaryRegistry(t, []*command{
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

	if err := registry.ValidateCommands(); err != nil {
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
			registry := withTemporaryRegistry(t, tt.cmds)
			if err := registry.ValidateCommands(); err == nil {
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

	GetRegistry().HandleInteraction(session, interaction)
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

	GetRegistry().HandleInteraction(session, interaction)
	if !called {
		t.Fatalf("expected interaction callback request")
	}
	cmd, exists := GetRegistry().getCmdFromKey(Hello)
	if !exists || cmd == nil {
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

	GetRegistry().HandleInteraction(session, interaction)
	if !called {
		t.Fatalf("expected interaction callback request")
	}
}

func TestHandleInteractionExecuteError(t *testing.T) {
	registry := withTemporaryRegistry(t, []*command{
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

	registry.HandleInteraction(session, interaction)
	if called {
		t.Fatalf("did not expect interaction callback request when execute fails")
	}
}

func TestExecuteFromKey(t *testing.T) {
	registry := withTemporaryRegistry(t, []*command{
		{
			Key: Hello,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "hello",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "hello response",
			},
		},
	})

	got, err := registry.ExecuteFromKey(Hello, discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello response" {
		t.Fatalf("unexpected response: %q", got)
	}

	if _, err := registry.ExecuteFromKey(CommandKey("missing"), discordgo.EnglishUS); !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestExecuteFromContentAndName(t *testing.T) {
	registry := withTemporaryRegistry(t, []*command{
		{
			Key: Ping,
			Descriptions: map[discordgo.Locale]string{
				discordgo.EnglishUS: "ping",
			},
			ResponseBuilder: simpleResponse,
			ResponseTemplate: map[discordgo.Locale]string{
				discordgo.EnglishUS: "pong",
			},
		},
	})

	got, err := registry.ExecuteFromName("  PING ", discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error from name: %v", err)
	}
	if got != "pong" {
		t.Fatalf("unexpected response from name: %q", got)
	}

	got, err = registry.ExecuteFromContent("  ping now", discordgo.EnglishUS)
	if err != nil {
		t.Fatalf("unexpected error from content: %v", err)
	}
	if got != "pong" {
		t.Fatalf("unexpected response from content: %q", got)
	}

	if _, err := registry.ExecuteFromContent("   ", discordgo.EnglishUS); !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound for empty content, got %v", err)
	}
}
