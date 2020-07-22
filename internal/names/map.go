package names

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/diamondburned/arikawa/discord"
)

// Map is a collection of mappings from IRC names to Discord IDs.
//
// It contains bidirectional maps for:
//      - Usernames to Discord users
//      - Nicknames to Discord users
//      - Channnels to Discord channels
type Map struct {
	mu     sync.Mutex
	userf  map[discord.UserID]string
	userb  map[string]discord.UserID
	nicksf map[discord.GuildID]map[discord.UserID]string
	nicksb map[discord.GuildID]map[string]discord.UserID
	chansf map[discord.GuildID]map[discord.ChannelID]string
	chansb map[discord.GuildID]map[string]discord.ChannelID
}

// NewMap returns a Map ready for use.
//
// A Map must not be copied after first use.
func NewMap() *Map {
	return &Map{
		userf:  make(map[discord.UserID]string),
		userb:  make(map[string]discord.UserID),
		nicksf: make(map[discord.GuildID]map[discord.UserID]string),
		nicksb: make(map[discord.GuildID]map[string]discord.UserID),
		chansf: make(map[discord.GuildID]map[discord.ChannelID]string),
		chansb: make(map[discord.GuildID]map[string]discord.ChannelID),
	}
}

// UserName returns the IRC name for userID.
//
// userID must be a valid snowflake.
func (m *Map) UserName(userID discord.UserID) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !userID.Valid() {
		panic("UserName: invalid userID")
	}

	return m.userf[userID]
}

// UserID returns the Discord ID for name.
//
// name must not be empty.
func (m *Map) UserID(name string) discord.UserID {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		panic("UserID: invalid name")
	}

	return m.userb[name]
}

// NickName returns the IRC name for userID in guildID.
//
// guildID must be a valid snowflake or the zero snowflake.
// userID must be a valid snowflake.
func (m *Map) NickName(guildID discord.GuildID, userID discord.UserID) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !guildID.Valid() && guildID != 0 {
		panic("NickName: invalid guildID")
	}

	if !userID.Valid() {
		panic("NickName: invalid userID")
	}

	nickmap, ok := m.nicksf[guildID]
	if !ok {
		return ""
	}

	return nickmap[userID]
}

// NickID returns the Discord ID for nick in guildID.
//
// guildID must be a valid snowflake or the zero snowflake.
// nick must not be empty.
func (m *Map) NickID(guildID discord.GuildID, nick string) discord.UserID {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !guildID.Valid() && guildID != 0 {
		panic("NickID: invalid guildID")
	}

	if nick == "" {
		panic("NickID: invalid nick")
	}

	nickmap, ok := m.nicksb[guildID]
	if !ok {
		return discord.UserID(0)
	}

	return nickmap[nick]
}

// ChannelName returns the IRC name for channelID in guildID.
//
// guildID must be a valid snowflake or the zero snowflake.
// channelID must be a valid snowflake.
func (m *Map) ChannelName(guildID discord.GuildID,
	channelID discord.ChannelID) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !guildID.Valid() && guildID != 0 {
		panic("ChannelName: invalid guildID")
	}

	if !channelID.Valid() {
		panic("ChannelName: invalid channelID")
	}

	chanmap, ok := m.chansf[guildID]
	if !ok {
		return ""
	}

	return chanmap[channelID]
}

// ChannelID returns the Discord ID for channel in guildID.
//
// guildID must be a valid snowflake or the zero snowflake.
// channelID must not be empty.
func (m *Map) ChannelID(guildID discord.GuildID,
	channel string) discord.ChannelID {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !guildID.Valid() && guildID != 0 {
		panic("ChannelID: invalid guildID")
	}

	if channel == "" {
		panic("ChannelID: invalid channel")
	}

	chanmap, ok := m.chansb[guildID]
	if !ok {
		return discord.ChannelID(0)
	}

	return chanmap[channel]
}

// UpdateUser updates the username entry for userID with ideal.
// It returns the previous value and the current value as before and current
// respectively.
//
// To remove an entry, pass an empty string as ideal.
// If before is empty, userID was not in the map.
// If current is empty, userID is no longer in the map.
func (m *Map) UpdateUser(userID discord.UserID,
	ideal string) (before string, current string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentName, ok := m.userf[userID]
	if ok && demangle(currentName) == ideal {
		return currentName, currentName
	}

	newName := ideal

	for {
		_, ok := m.userb[newName]
		if !ok {
			break
		}
		newName = mangle(newName, int64(userID))
	}

	m.userf[userID] = newName
	m.userb[newName] = userID

	if ok {
		delete(m.userb, currentName)
	}

	return currentName, newName
}

// UpdateNickLock updates the nickname entry for userID in nicksf and nicksb
// with ideal. It returns the previous value and the current value as before
// and current respectively.
//
// To remove an entry, pass an empty string as ideal.
// If before is empty, userID was not in the map.
// If current is empty, userID is no longer in the map.
//
// nicksf and nicksb must not be nil.
// The caller must hold m.mu.
func (m *Map) updateNickLock(
	nicksf map[discord.UserID]string, nicksb map[string]discord.UserID,
	userID discord.UserID, ideal string) (before string, current string) {
	currentName, ok := nicksf[userID]
	if ok && demangle(currentName) == ideal {
		return currentName, currentName
	}

	newName := ideal

	for {
		_, ok := nicksb[newName]
		if !ok {
			break
		}
		newName = mangle(newName, int64(userID))
	}

	nicksf[userID] = newName
	nicksb[newName] = userID

	if ok {
		delete(m.userb, currentName)
	}

	return currentName, newName
}

// UpdateChannelLock updates the channel name entry for channelID in chansf and
// chansb with ideal. It returns the previous value and the current value as
// before and current respectively.
//
// To remove an entry, pass an empty string as ideal.
// If before is empty, channelID was not in the map.
// If current is empty, channelID is no longer in the map.
//
// chansf and chansb must not be nil.
// The caller must hold m.mu.
func (m *Map) updateChannelLock(chansf map[discord.ChannelID]string,
	chansb map[string]discord.ChannelID,
	channelID discord.ChannelID,
	ideal string) (before string, current string) {
	currentName, ok := chansf[channelID]
	if ok && demangle(currentName) == ideal {
		return currentName, currentName
	}

	newName := ideal

	for {
		_, ok := chansb[newName]
		if !ok {
			break
		}
		newName = mangle(newName, int64(channelID))
	}

	chansf[channelID] = newName
	chansb[newName] = channelID

	if ok {
		delete(m.userb, currentName)
	}

	return currentName, newName
}

// UpdateNick updates the nickname entry for userID in guildID with ideal.
// It returns the previous value and the current value as before and current
// respectively.
//
// To remove an entry, pass an empty string as ideal.
// If before is empty, userID was not in the map.
// If current is empty, userID is no longer in the map.
func (m *Map) UpdateNick(guildID discord.GuildID,
	userID discord.UserID, ideal string) (before string, current string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nicksf[guildID]; !ok {
		m.nicksf[guildID] = make(map[discord.UserID]string)
	}

	if _, ok := m.nicksb[guildID]; !ok {
		m.nicksb[guildID] = make(map[string]discord.UserID)
	}

	return m.updateNickLock(
		m.nicksf[guildID], m.nicksb[guildID], userID, ideal)
}

// UpdateChannel updates the channel name entry for channelID in guildID
// with ideal. It returns the previous value and the current value as before
// and current respectively.
//
// To remove an entry, pass an empty string as ideal.
// If before is empty, channelID was not in the map.
// If current is empty, channelID is no longer in the map.
func (m *Map) UpdateChannel(guildID discord.GuildID,
	channelID discord.ChannelID,
	ideal string) (before string, current string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.chansf[guildID]; !ok {
		m.chansf[guildID] = make(map[discord.ChannelID]string)
	}

	if _, ok := m.chansb[guildID]; !ok {
		m.chansb[guildID] = make(map[string]discord.ChannelID)
	}

	return m.updateChannelLock(
		m.chansf[guildID], m.chansb[guildID], channelID, ideal)
}

// sanitize returns name with all instances of
// the mangling separator '#' removed.
func sanitize(name string) string {
	return strings.ReplaceAll(name, "#", "")
}

// mangle creates a distinct string based on name and id.
//
// mangle always returns a string different from name.
// Calling mangle in a loop with the result of the previous invocation
// on the result will never produce a duplicate string.
func mangle(name string, id int64) string {
	idstr := strconv.FormatInt(id, 10)

	if i := strings.IndexRune(name, '#'); i != -1 {
		newlen := len(name) - i
		for len(idstr) < newlen {
			idstr += "#"
		}
		return fmt.Sprintf("%s#%s", name[:i], idstr[:newlen])
	}

	return fmt.Sprintf("%s#%c", name, idstr[0])
}

// demangle undoes the result of mangle on name, no matter how many
// rounds of mangling name received.
func demangle(name string) string {
	if i := strings.IndexRune(name, '#'); i != -1 {
		return name[:i]
	}

	return name
}
