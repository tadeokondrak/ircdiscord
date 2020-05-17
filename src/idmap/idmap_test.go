package idmap

import (
	"testing"

	"github.com/diamondburned/arikawa/discord"
)

func assert(cond bool) {
	if !cond {
		panic("assertion failed")
	}
}

func TestMangle(t *testing.T) {
	assert(mangle("name", 12345) == "name#1")
	assert(mangle("name#1", 12345) == "name#12")
	assert(mangle("name#12", 12345) == "name#123")
	assert(mangle("name#123", 12345) == "name#1234")
	assert(mangle("name#1234", 12345) == "name#12345")
	assert(mangle("name#12345", 12345) == "name#12345#")
	assert(mangle("name#12345#", 12345) == "name#12345##")
	assert(mangle("name#12345##", 12345) == "name#12345###")
}

func TestIDMap(t *testing.T) {
	m := New()
	assert(m.Name(discord.Snowflake(12345)) == "")
	assert(m.Insert(discord.Snowflake(12345), "name") == "name")
	assert(m.Insert(discord.Snowflake(12346), "name") == "name#1")
	assert(m.Insert(discord.Snowflake(12347), "name") == "name#12")
	assert(m.Name(discord.Snowflake(12345)) == "name")
	assert(m.Name(discord.Snowflake(12346)) == "name#1")
	assert(m.Name(discord.Snowflake(12347)) == "name#12")
	assert(m.Snowflake("name") == discord.Snowflake(12345))
	assert(m.Snowflake("name#1") == discord.Snowflake(12346))
	assert(m.Snowflake("name#12") == discord.Snowflake(12347))
}
