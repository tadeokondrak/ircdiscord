package main

import (
	"strconv"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type snowflakeMap struct {
	table map[string]string
	sync.Mutex
}

func newSnowflakeMap() *snowflakeMap {
	return &snowflakeMap{
		table: map[string]string{},
	}
}

func (m *snowflakeMap) add(name string, snowflake string, separator string) string {
	m.Lock()
	defer m.Unlock()
	var suffix string
	for i := 0; ; i++ {
		if i == 0 {
			suffix = ""
		} else {
			suffix = separator + strconv.Itoa(i)
		}
		_name := name + suffix
		_, exists := m.table[_name]
		if !exists {
			m.table[_name] = snowflake
			return _name
		}
	}
}

func (m *snowflakeMap) get(name string) string {
	snowflake, exists := m.table[name]
	if exists {
		return snowflake
	}
	return ""
}

func (m *snowflakeMap) getMap() map[string]string {
	return m.table
}

func (m *snowflakeMap) getFromSnowflake(snowflake string) string {
	for name, _snowflake := range m.table {
		if _snowflake == snowflake {
			return name
		}
	}
	return ""
}

func (m *snowflakeMap) removeFromSnowflake(snowflake string) string {
	name := m.getFromSnowflake(snowflake)
	m.remove(name)
	return name
}

func (m *snowflakeMap) remove(name string) {
	delete(m.table, name)
}

func (m *snowflakeMap) clear() {
	m = newSnowflakeMap()
}

func (m *snowflakeMap) addChannel(channel *discordgo.Channel) string {
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeDM {
		return ""
	}
	return m.add(convertDiscordChannelNameToIRC(channel.Name), channel.ID, "#")
}

func (m *snowflakeMap) addUser(user *discordgo.User) string {
	if user.Discriminator == "0000" {
		// We don't add users for webhooks because they're not users
		return ""
	}
	return m.add(convertDiscordUsernameToIRCNick(user.Username), user.ID, "@")
}

func (m *snowflakeMap) getNick(discordUser *discordgo.User) string {
	if discordUser == nil {
		return ""
	}
	username := convertDiscordUsernameToIRCNick(discordUser.Username)
	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return username + "@w"
	}
	return username
}
