package router

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func newTestSessionWithBotID(t *testing.T, botID string) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new discord session: %v", err)
	}
	session.State.User = &discordgo.User{ID: botID}

	return session
}

func TestResolveMessageRouteMentionBeforeGuild(t *testing.T) {
	session := newTestSessionWithBotID(t, "bot-id")

	route := ResolveMessageRoute(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID:  "guild-1",
		Mentions: []*discordgo.User{{ID: "other"}, {ID: "bot-id"}},
	}})
	if route != RouteMention {
		t.Fatalf("expected mention route, got %v", route)
	}
}

func TestResolveMessageRouteDirect(t *testing.T) {
	session := newTestSessionWithBotID(t, "bot-id")

	route := ResolveMessageRoute(session, &discordgo.MessageCreate{Message: &discordgo.Message{}})
	if route != RouteDirect {
		t.Fatalf("expected direct route, got %v", route)
	}
}

func TestResolveMessageRouteGuild(t *testing.T) {
	session := newTestSessionWithBotID(t, "bot-id")

	route := ResolveMessageRoute(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1",
		Author:  &discordgo.User{ID: "user-1"},
	}})
	if route != RouteGuild {
		t.Fatalf("expected guild route, got %v", route)
	}
}
