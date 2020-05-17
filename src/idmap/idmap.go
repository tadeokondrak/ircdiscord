package idmap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/discord"
)

// IDMap is a map between Discord ID/name pairs and IRC names.
// There may be multiple Discord names with the same name but different IDs,
// with different IRC names guaranteed by the map.
// It is not safe to use concurrently from multiple goroutines.
type IDMap struct {
	forward  map[discord.Snowflake]string
	backward map[string]discord.Snowflake
}

// New creates a new IDMap.
func New() *IDMap {
	return &IDMap{
		forward:  make(map[discord.Snowflake]string),
		backward: make(map[string]discord.Snowflake),
	}
}

// Name returns the IRC name for a Discord ID.
// It returns the empty string if missing.
func (m *IDMap) Name(id discord.Snowflake) string {
	if !id.Valid() {
		panic("invalid ID")
	}
	if name, ok := m.forward[id]; ok {
		return name
	}
	return ""
}

// Snowflake returns the Discord ID from an IRC name.
// It returns the null snowflake if missing.
func (m *IDMap) Snowflake(name string) discord.Snowflake {
	if name == "" {
		panic("invalid IRC name")
	}
	if flake, ok := m.backward[name]; ok {
		return flake
	}
	return discord.NullSnowflake
}

// Insert returns an IRC name for a given Discord ID.
// It returns ideal if there were no collisions.
func (m *IDMap) Insert(id discord.Snowflake, ideal string) string {
	if name := m.Name(id); name != "" {
		return name
	}
	name := sanitize(ideal)
	for {
		_, ok := m.backward[name]
		if ok {
			name = mangle(name, int64(id))
			continue
		}
		break
	}
	m.forward[id] = name
	m.backward[name] = id
	return name
}

// TODO: add Remove

var hashReplacer = strings.NewReplacer("#", "")

func sanitize(name string) string {
	return hashReplacer.Replace(name)
}

func mangle(name string, id int64) string {
	idStr := strconv.FormatInt(id, 10)
	if i := strings.IndexRune(name, '#'); i != -1 {
		newlen := len(name) - i
		// inefficient but this will hopefully never be hit
		for len(idStr) < newlen {
			idStr += "#"
		}
		return fmt.Sprintf("%s#%s", name[:i], idStr[:newlen])
	} else {
		return fmt.Sprintf("%s#%c", name, idStr[0])
	}
}
