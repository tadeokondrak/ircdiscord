package names

import (
	"fmt"
	"testing"

	"github.com/diamondburned/arikawa/discord"
	"github.com/stretchr/testify/assert"
)

func TestMap_UserName(t *testing.T) {

	m := NewMap()

	assert.Equal(t, "", m.UserName(discord.UserID(1)))

	m.UpdateUser(discord.UserID(1), "one")
	m.UpdateUser(discord.UserID(2), "two")
	m.UpdateUser(discord.UserID(3), "three")

	assert.PanicsWithValue(t, "UserName: invalid userID", func() {
		m.UserName(discord.UserID(0))
	})
	assert.Equal(t, "one", m.UserName(discord.UserID(1)))
	assert.Equal(t, "two", m.UserName(discord.UserID(2)))
	assert.Equal(t, "three", m.UserName(discord.UserID(3)))

	m.UpdateUser(discord.UserID(3), "")
	m.UpdateUser(discord.UserID(4), "four")
	m.UpdateUser(discord.UserID(5), "one")
	m.UpdateUser(discord.UserID(6), "one")
	m.UpdateUser(discord.UserID(66), "one")
	m.UpdateUser(discord.UserID(66), "one")

	assert.Equal(t, "one", m.UserName(discord.UserID(1)))
	assert.Equal(t, "two", m.UserName(discord.UserID(2)))
	assert.Equal(t, "", m.UserName(discord.UserID(3)))
	assert.Equal(t, "four", m.UserName(discord.UserID(4)))
	assert.Equal(t, "one#5", m.UserName(discord.UserID(5)))
	assert.Equal(t, "one#6", m.UserName(discord.UserID(6)))
	assert.Equal(t, "one#66", m.UserName(discord.UserID(66)))

	m.UpdateUser(discord.UserID(66), "two")
	assert.Equal(t, "two#6", m.UserName(discord.UserID(66)))
}

func TestMap_UserID(t *testing.T) {
	m := NewMap()

	assert.Equal(t, discord.UserID(0), m.UserID("zero"))

	m.UpdateUser(discord.UserID(1), "one")
	m.UpdateUser(discord.UserID(2), "two")
	m.UpdateUser(discord.UserID(3), "three")

	assert.PanicsWithValue(t, "UserID: invalid name", func() {
		m.UserID("")
	})
	assert.Equal(t, discord.UserID(1), m.UserID("one"))
	assert.Equal(t, discord.UserID(2), m.UserID("two"))
	assert.Equal(t, discord.UserID(3), m.UserID("three"))

	m.UpdateUser(discord.UserID(3), "")
	m.UpdateUser(discord.UserID(4), "four")
	m.UpdateUser(discord.UserID(5), "one")
	m.UpdateUser(discord.UserID(6), "one")
	m.UpdateUser(discord.UserID(66), "one")

	assert.Equal(t, discord.UserID(1), m.UserID("one"))
	assert.Equal(t, discord.UserID(2), m.UserID("two"))
	assert.Equal(t, discord.UserID(4), m.UserID("four"))
	assert.Equal(t, discord.UserID(5), m.UserID("one#5"))
	assert.Equal(t, discord.UserID(6), m.UserID("one#6"))
	assert.Equal(t, discord.UserID(66), m.UserID("one#66"))
}

func TestMap_NickName(t *testing.T) {
	t.Run("panic on invalid guild", func(t *testing.T) {
		t.Parallel()
		m := NewMap()

		assert.PanicsWithValue(t, "NickName: invalid guildID", func() {
			m.NickName(
				discord.GuildID(discord.NullSnowflake),
				discord.UserID(1))
		})
	})

	f := func(t *testing.T, g discord.GuildID) {
		t.Parallel()
		m := NewMap()

		assert.Equal(t, "", m.NickName(g, discord.UserID(1)))

		m.UpdateNick(g, discord.UserID(1), "one")
		m.UpdateNick(g, discord.UserID(2), "two")
		m.UpdateNick(g, discord.UserID(3), "three")

		assert.PanicsWithValue(t, "NickName: invalid userID", func() {
			m.NickName(g, discord.UserID(0))
		})
		assert.Equal(t, "one", m.NickName(g, discord.UserID(1)))
		assert.Equal(t, "two", m.NickName(g, discord.UserID(2)))
		assert.Equal(t, "three", m.NickName(g, discord.UserID(3)))

		m.UpdateNick(g, discord.UserID(3), "")
		m.UpdateNick(g, discord.UserID(4), "four")
		m.UpdateNick(g, discord.UserID(5), "one")
		m.UpdateNick(g, discord.UserID(6), "one")
		m.UpdateNick(g, discord.UserID(66), "one")
		m.UpdateNick(g, discord.UserID(66), "one")

		assert.Equal(t, "one", m.NickName(g, discord.UserID(1)))
		assert.Equal(t, "two", m.NickName(g, discord.UserID(2)))
		assert.Equal(t, "", m.NickName(g, discord.UserID(3)))
		assert.Equal(t, "four", m.NickName(g, discord.UserID(4)))
		assert.Equal(t, "one#5", m.NickName(g, discord.UserID(5)))
		assert.Equal(t, "one#6", m.NickName(g, discord.UserID(6)))
		assert.Equal(t, "one#66", m.NickName(g, discord.UserID(66)))

		m.UpdateNick(g, discord.UserID(66), "two")
		assert.Equal(t, "two#6", m.NickName(g, discord.UserID(66)))
	}

	for i := 0; i < 2; i++ {
		g := discord.GuildID(i)
		t.Run(fmt.Sprintf("id %d", i), func(t *testing.T) { f(t, g) })
	}
}

func TestMap_NickID(t *testing.T) {
	t.Run("panic on invalid guild", func(t *testing.T) {
		t.Parallel()
		m := NewMap()

		assert.PanicsWithValue(t, "NickID: invalid guildID", func() {
			m.NickID(discord.GuildID(discord.NullSnowflake), "zero")
		})
	})

	f := func(t *testing.T, g discord.GuildID) {
		t.Parallel()
		m := NewMap()

		assert.Equal(t, discord.UserID(0), m.NickID(g, "zero"))

		m.UpdateNick(g, discord.UserID(1), "one")
		m.UpdateNick(g, discord.UserID(2), "two")
		m.UpdateNick(g, discord.UserID(3), "three")

		assert.PanicsWithValue(t, "NickID: invalid nick", func() {
			m.NickID(g, "")
		})
		assert.Equal(t, discord.UserID(1), m.NickID(g, "one"))
		assert.Equal(t, discord.UserID(2), m.NickID(g, "two"))
		assert.Equal(t, discord.UserID(3), m.NickID(g, "three"))

		m.UpdateNick(g, discord.UserID(3), "")
		m.UpdateNick(g, discord.UserID(4), "four")
		m.UpdateNick(g, discord.UserID(5), "one")
		m.UpdateNick(g, discord.UserID(6), "one")
		m.UpdateNick(g, discord.UserID(66), "one")

		assert.Equal(t, discord.UserID(1), m.NickID(g, "one"))
		assert.Equal(t, discord.UserID(2), m.NickID(g, "two"))
		assert.Equal(t, discord.UserID(4), m.NickID(g, "four"))
		assert.Equal(t, discord.UserID(5), m.NickID(g, "one#5"))
		assert.Equal(t, discord.UserID(6), m.NickID(g, "one#6"))
		assert.Equal(t, discord.UserID(66), m.NickID(g, "one#66"))
	}

	for i := 0; i < 2; i++ {
		g := discord.GuildID(i)
		t.Run(fmt.Sprintf("id %d", i), func(t *testing.T) { f(t, g) })
	}
}

func TestMap_ChannelName(t *testing.T) {
	t.Run("panic on invalid guild", func(t *testing.T) {
		t.Parallel()
		m := NewMap()

		assert.PanicsWithValue(t, "ChannelName: invalid guildID", func() {
			m.ChannelName(
				discord.GuildID(discord.NullSnowflake),
				discord.ChannelID(1))
		})
	})

	f := func(t *testing.T, g discord.GuildID) {
		t.Parallel()
		m := NewMap()

		assert.Equal(t, "", m.ChannelName(g, discord.ChannelID(1)))

		m.UpdateChannel(g, discord.ChannelID(1), "one")
		m.UpdateChannel(g, discord.ChannelID(2), "two")
		m.UpdateChannel(g, discord.ChannelID(3), "three")

		assert.PanicsWithValue(t, "ChannelName: invalid channelID", func() {
			m.ChannelName(g, discord.ChannelID(0))
		})
		assert.Equal(t, "one", m.ChannelName(g, discord.ChannelID(1)))
		assert.Equal(t, "two", m.ChannelName(g, discord.ChannelID(2)))
		assert.Equal(t, "three", m.ChannelName(g, discord.ChannelID(3)))

		m.UpdateChannel(g, discord.ChannelID(3), "")
		m.UpdateChannel(g, discord.ChannelID(4), "four")
		m.UpdateChannel(g, discord.ChannelID(5), "one")
		m.UpdateChannel(g, discord.ChannelID(6), "one")
		m.UpdateChannel(g, discord.ChannelID(66), "one")
		m.UpdateChannel(g, discord.ChannelID(66), "one")

		assert.Equal(t, "one", m.ChannelName(g, discord.ChannelID(1)))
		assert.Equal(t, "two", m.ChannelName(g, discord.ChannelID(2)))
		assert.Equal(t, "", m.ChannelName(g, discord.ChannelID(3)))
		assert.Equal(t, "four", m.ChannelName(g, discord.ChannelID(4)))
		assert.Equal(t, "one#5", m.ChannelName(g, discord.ChannelID(5)))
		assert.Equal(t, "one#6", m.ChannelName(g, discord.ChannelID(6)))
		assert.Equal(t, "one#66", m.ChannelName(g, discord.ChannelID(66)))

		m.UpdateChannel(g, discord.ChannelID(66), "two")
		assert.Equal(t, "two#6", m.ChannelName(g, discord.ChannelID(66)))
	}

	for i := 0; i < 2; i++ {
		g := discord.GuildID(i)
		t.Run(fmt.Sprintf("id %d", i), func(t *testing.T) { f(t, g) })
	}
}

func TestMap_ChannelID(t *testing.T) {
	t.Run("panic on invalid guild", func(t *testing.T) {
		t.Parallel()
		m := NewMap()

		assert.PanicsWithValue(t, "ChannelID: invalid guildID", func() {
			m.ChannelID(discord.GuildID(discord.NullSnowflake), "zero")
		})
	})

	f := func(t *testing.T, g discord.GuildID) {
		t.Parallel()
		m := NewMap()

		assert.Equal(t, discord.ChannelID(0), m.ChannelID(g, "zero"))

		m.UpdateChannel(g, discord.ChannelID(1), "one")
		m.UpdateChannel(g, discord.ChannelID(2), "two")
		m.UpdateChannel(g, discord.ChannelID(3), "three")

		assert.PanicsWithValue(t, "ChannelID: invalid channel", func() {
			m.ChannelID(g, "")
		})
		assert.Equal(t, discord.ChannelID(1), m.ChannelID(g, "one"))
		assert.Equal(t, discord.ChannelID(2), m.ChannelID(g, "two"))
		assert.Equal(t, discord.ChannelID(3), m.ChannelID(g, "three"))

		m.UpdateChannel(g, discord.ChannelID(3), "")
		m.UpdateChannel(g, discord.ChannelID(4), "four")
		m.UpdateChannel(g, discord.ChannelID(5), "one")
		m.UpdateChannel(g, discord.ChannelID(6), "one")
		m.UpdateChannel(g, discord.ChannelID(66), "one")

		assert.Equal(t, discord.ChannelID(1), m.ChannelID(g, "one"))
		assert.Equal(t, discord.ChannelID(2), m.ChannelID(g, "two"))
		assert.Equal(t, discord.ChannelID(4), m.ChannelID(g, "four"))
		assert.Equal(t, discord.ChannelID(5), m.ChannelID(g, "one#5"))
		assert.Equal(t, discord.ChannelID(6), m.ChannelID(g, "one#6"))
		assert.Equal(t, discord.ChannelID(66), m.ChannelID(g, "one#66"))
	}

	for i := 0; i < 2; i++ {
		g := discord.GuildID(i)
		t.Run(fmt.Sprintf("id %d", i), func(t *testing.T) { f(t, g) })
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		Input    string
		Expected string
	}{
		{"name", "name"},
		{"name#1", "name1"},
		{"name#12", "name12"},
		{"name#123", "name123"},
		{"name#123#", "name123"},
	}

	for _, test := range tests {
		test := test
		f := func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.Expected,
				sanitize(test.Input))
		}
		t.Run(fmt.Sprintf("sanitize %s", test.Input), f)
	}
}

func TestMangle(t *testing.T) {
	tests := []struct {
		Name     string
		ID       int64
		Expected string
	}{
		{"name", 123, "name#1"},
		{"name#1", 123, "name#12"},
		{"name#12", 123, "name#123"},
		{"name#123", 123, "name#123#"},
		{"name#123#", 123, "name#123##"},
	}

	for _, test := range tests {
		test := test
		f := func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.Expected,
				mangle(test.Name, test.ID))
		}
		t.Run(fmt.Sprintf("mangle %s %d", test.Name, test.ID), f)
	}
}

func TestDemangle(t *testing.T) {
	tests := []struct{ Input, Expected string }{
		{"name", "name"},
		{"name#1", "name"},
		{"name#12", "name"},
		{"name#123", "name"},
		{"name#123#", "name"},
	}

	for _, test := range tests {
		test := test
		f := func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.Expected, demangle(test.Input))
		}
		t.Run(fmt.Sprintf("demangle %s", test.Input), f)
	}
}
