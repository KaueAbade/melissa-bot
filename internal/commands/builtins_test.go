package commands

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestGetCommandDefinitionsIncludesExpectedBuiltinKeys(t *testing.T) {
	definitions := getCommandDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("expected 4 builtin command definitions, got %d", len(definitions))
	}

	expected := map[CommandKey]struct{}{
		Help:  {},
		Hello: {},
		Ping:  {},
		Roll:  {},
	}

	seen := make(map[CommandKey]struct{}, len(definitions))
	for _, cmd := range definitions {
		if cmd == nil {
			t.Fatalf("expected non-nil builtin command")
		}
		if _, ok := expected[cmd.Key]; !ok {
			t.Fatalf("unexpected builtin command key: %s", cmd.Key)
		}
		if _, duplicated := seen[cmd.Key]; duplicated {
			t.Fatalf("duplicated builtin command key: %s", cmd.Key)
		}
		seen[cmd.Key] = struct{}{}
	}

	for key := range expected {
		if _, ok := seen[key]; !ok {
			t.Fatalf("missing expected builtin command key: %s", key)
		}
	}
}

func TestGetCommandDefinitionsHaveRequiredDefaultFields(t *testing.T) {
	for _, cmd := range getCommandDefinitions() {
		if cmd.ResponseBuilder == nil {
			t.Fatalf("command %s has nil response builder", cmd.Key)
		}
		if len(cmd.Descriptions) == 0 {
			t.Fatalf("command %s has no descriptions", cmd.Key)
		}
		if len(cmd.ResponseTemplate) == 0 {
			t.Fatalf("command %s has no response templates", cmd.Key)
		}

		description := cmd.Descriptions[discordgo.EnglishUS]
		if description == "" {
			t.Fatalf("command %s has empty default description", cmd.Key)
		}

		response := cmd.ResponseTemplate[discordgo.EnglishUS]
		if response == "" {
			t.Fatalf("command %s has empty default response", cmd.Key)
		}
	}
}
