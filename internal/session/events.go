package session

import "github.com/diamondburned/arikawa/discord"

type UserNameChange struct {
	GuildID   discord.Snowflake
	ID        discord.Snowflake
	Old       string // can be empty
	New       string // can be empty
	IsInitial bool
}

type ChannelNameChange struct {
	ID  discord.Snowflake
	Old string
	New string
}
