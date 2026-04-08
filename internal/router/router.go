package router

import (
	"github.com/bwmarrin/discordgo"
)

// Route represents the type of message route for incoming Discord messages.
type Route int

const (
	RouteMention Route = iota
	RouteDirect
	RouteGuild
)

// ResolveMessageRoute determines the route type for a given message based on its content and context.
func ResolveMessageRoute(session *discordgo.Session, message *discordgo.MessageCreate) Route {
	if message.Mentions != nil {
		for _, user := range message.Mentions {
			if user.ID == session.State.User.ID {
				return RouteMention
			}
		}
	}

	if message.GuildID == "" {
		return RouteDirect
	}

	return RouteGuild
}
