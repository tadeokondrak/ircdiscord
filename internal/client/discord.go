package client

import (
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/internal/render"
	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"gopkg.in/irc.v3"
)

func (c *Client) discordUserPrefix(u *discord.User) *irc.Prefix {
	return &irc.Prefix{
		User: c.session.UserName(u.ID, u.Username),
		Name: c.session.UserName(u.ID, u.Username),
		Host: u.ID.String(),
	}
}

func (c *Client) sendDiscordMessage(m *discord.Message) error {
	// TODO: arikawa should store relationships in its state
	for _, rel := range c.session.Ready.Relationships {
		if rel.User.ID == m.Author.ID && rel.Type == gateway.BlockedRelationship {
			return nil
		}
	}

	channel, err := c.session.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	var channelName string

	if c.guild.Valid() {
		var err error
		channelName, err = c.session.ChannelName(m.GuildID, m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		recip := channel.DMRecipients[0]
		channelName = c.session.UserName(recip.ID, recip.Username)
	}

	if !c.guild.Valid() {
		c.sendJoin(channel)
	}

	message, err := render.Message(c.session, m)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(message, "\n") {
		if err := replies.PRIVMSG(
			c, m.ID.Time(), c.discordUserPrefix(&m.Author), channelName, line,
		); err != nil {
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
		// TODO: update map
	case *gateway.ChannelUpdateEvent:
		// TODO: update map
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
	if c.guild.Valid() {
		if m.GuildID != c.guild {
			return nil
		}
	} else {
		channel, err := c.session.Channel(m.ChannelID)
		if err != nil {
			return err
		}
		if channel.Type != discord.DirectMessage {
			return nil
		}
	}
	if m.ID == c.lastMessageID && !c.HasCapability("echo-message") {
		return nil
	}
	return c.sendDiscordMessage(m)
}
