package idmap

import (
	"testing"

	"github.com/diamondburned/arikawa/discord"
	"github.com/stretchr/testify/assert"
)

func TestMangle(t *testing.T) {
	assert.Equal(t, mangle("name", 12345), "name#1")
	assert.Equal(t, mangle("name#1", 12345), "name#12")
	assert.Equal(t, mangle("name#12", 12345), "name#123")
	assert.Equal(t, mangle("name#123", 12345), "name#1234")
	assert.Equal(t, mangle("name#1234", 12345), "name#12345")
	assert.Equal(t, mangle("name#12345", 12345), "name#12345#")
	assert.Equal(t, mangle("name#12345#", 12345), "name#12345##")
	assert.Equal(t, mangle("name#12345##", 12345), "name#12345###")
}

func TestIDMap(t *testing.T) {
	m := New()

	var oldname, newname string
	assert.Equal(t, m.Name(discord.Snowflake(12345)), "")

	oldname, newname = m.Insert(discord.Snowflake(12345), "name")
	assert.Equal(t, oldname, "")
	assert.Equal(t, newname, "name")

	oldname, newname = m.Insert(discord.Snowflake(12346), "name")
	assert.Equal(t, oldname, "")
	assert.Equal(t, newname, "name#1")

	oldname, newname = m.Insert(discord.Snowflake(12347), "name")
	assert.Equal(t, oldname, "")
	assert.Equal(t, newname, "name#12")

	assert.Equal(t, m.Name(discord.Snowflake(12345)), "name")
	assert.Equal(t, m.Name(discord.Snowflake(12346)), "name#1")
	assert.Equal(t, m.Name(discord.Snowflake(12347)), "name#12")

	assert.Equal(t, m.Snowflake("name"), discord.Snowflake(12345))
	assert.Equal(t, m.Snowflake("name#1"), discord.Snowflake(12346))
	assert.Equal(t, m.Snowflake("name#12"), discord.Snowflake(12347))

	m.DeleteSnowflake(discord.Snowflake(12345))

	assert.False(t, m.Snowflake("name").Valid())
	assert.Equal(t, m.Snowflake("name#1"), discord.Snowflake(12346))
	assert.Equal(t, m.Snowflake("name#12"), discord.Snowflake(12347))

	oldname, newname = m.Insert(discord.Snowflake(12345), "name")
	assert.Equal(t, oldname, "")
	assert.Equal(t, newname, "name")
	assert.Equal(t, m.Name(discord.Snowflake(12345)), "name")
	assert.Equal(t, m.Snowflake("name"), discord.Snowflake(12345))

	assert.True(t, m.Snowflake("name#12").Valid())

	oldname, newname = m.Insert(discord.Snowflake(12347), "newname")
	assert.Equal(t, oldname, "name#12")
	assert.Equal(t, newname, "newname")
	assert.False(t, m.Snowflake("name#12").Valid())
	assert.True(t, m.Snowflake("newname").Valid())
}
