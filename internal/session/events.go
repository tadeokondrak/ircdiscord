package session

import "github.com/diamondburned/arikawa/discord"

type UserNameChange struct {
	GuildID discord.Snowflake
	ID      discord.Snowflake
	Old     string
	New     string
}

type ChannelNameChange struct {
	ID  discord.Snowflake
	Old string
	New string
}
