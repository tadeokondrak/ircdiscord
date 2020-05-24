package client

import (
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/internal/render"
	"gopkg.in/irc.v3"
)

func (c *Client) discordUserPrefix(u *discord.User) *irc.Prefix {
	return &irc.Prefix{
		User: c.session.UserName(u),
		Name: c.session.UserName(u),
		Host: u.ID.String(),
	}
}

func (c *Client) sendDiscordMessage(m *discord.Message) error {
	channelName, err := c.session.ChannelName(m.GuildID, m.ChannelID)
	if err != nil {
		return err
	}
	tags := make(irc.Tags)
	if c.caps["server-time"] {
		tags["time"] = irc.TagValue(m.ID.Time().UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	message, err := render.Message(c.session, m)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(message, "\n") {
		if err := c.irc.WriteMessage(&irc.Message{
			Tags:    tags,
			Prefix:  c.discordUserPrefix(&m.Author),
			Command: "PRIVMSG",
			Params:  []string{channelName, line},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) handleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.HelloEvent:
	case *gateway.ReadyEvent:
	case *gateway.ResumedEvent:
	case *gateway.InvalidSessionEvent:
	case *gateway.ChannelCreateEvent:
		if c.guild.Valid() {
			var channel discord.Channel = *(*discord.Channel)(e)
			if visible, err := c.channelIsVisible(&channel); err != nil {
				return err
			} else if visible {
				c.session.ChannelMap(c.guild).Insert(channel.ID, channel.Name)
			}
		}
	case *gateway.ChannelUpdateEvent:
		// TODO: remove from map
	case *gateway.ChannelDeleteEvent:
		// TODO: remove from map
	case *gateway.ChannelPinsUpdateEvent:
	case *gateway.ChannelUnreadUpdateEvent:
	case *gateway.GuildCreateEvent:
	case *gateway.GuildUpdateEvent:
	case *gateway.GuildDeleteEvent:
	case *gateway.GuildBanAddEvent:
	case *gateway.GuildBanRemoveEvent:
	case *gateway.GuildEmojisUpdateEvent:
	case *gateway.GuildIntegrationsUpdateEvent:
	case *gateway.GuildMemberAddEvent:
	case *gateway.GuildMemberRemoveEvent:
	case *gateway.GuildMemberUpdateEvent:
	case *gateway.GuildMembersChunkEvent:
	case *gateway.GuildMemberListUpdate:
	case *gateway.GuildRoleCreateEvent:
	case *gateway.GuildRoleUpdateEvent:
	case *gateway.GuildRoleDeleteEvent:
	case *gateway.InviteCreateEvent:
	case *gateway.InviteDeleteEvent:
	case *gateway.MessageCreateEvent:
		return c.handleDiscordMessage(&e.Message)
	case *gateway.MessageUpdateEvent:
		return c.handleDiscordMessage(&e.Message)
	case *gateway.MessageDeleteEvent:
	case *gateway.MessageDeleteBulkEvent:
	case *gateway.MessageReactionAddEvent:
	case *gateway.MessageReactionRemoveEvent:
	case *gateway.MessageReactionRemoveAllEvent:
	case *gateway.MessageAckEvent:
	case *gateway.PresenceUpdateEvent:
	case *gateway.PresencesReplaceEvent:
	case *gateway.SessionsReplaceEvent:
	case *gateway.TypingStartEvent:
	case *gateway.VoiceStateUpdateEvent:
	case *gateway.VoiceServerUpdateEvent:
	case *gateway.WebhooksUpdateEvent:
	case *gateway.UserSettingsUpdateEvent:
	case *gateway.UserGuildSettingsUpdateEvent:
	case *gateway.UserNoteUpdateEvent:
	}
	return nil
}

func (c *Client) handleDiscordMessage(m *discord.Message) error {
	if !c.guild.Valid() {
		return nil
	}
	if m.GuildID != c.guild {
		return nil
	}
	if m.ID == c.lastMessageID && !c.caps["echo-message"] {
		return nil
	}
	return c.sendDiscordMessage(m)
}
