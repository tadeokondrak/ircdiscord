package idmap

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/diamondburned/arikawa/discord"
)

// IDMap is a map between Discord ID/name pairs and IRC names.
//
// There may be multiple Discord names with the same name but different IDs,
// with different IRC names guaranteed by the map.
type IDMap struct {
	forward  map[discord.Snowflake]string
	backward map[string]discord.Snowflake
	mu       sync.RWMutex
}

// New creates a new IDMap.
func New() *IDMap {
	return &IDMap{
		forward:  make(map[discord.Snowflake]string),
		backward: make(map[string]discord.Snowflake),
	}
}

// Name returns the IRC name for a Discord ID.
//
// It returns the empty string if missing.
// It panics if passed an invalid snowflake.
func (m *IDMap) Name(id discord.Snowflake) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nameHeldRLock(id)
}

// Snowflake returns the Discord ID from an IRC name.
//
// It returns the zero snowflake if missing.
// It panics if passed the empty string.
func (m *IDMap) Snowflake(name string) discord.Snowflake {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snowflakeHeldRLock(name)
}

// DeleteSnowflake removes an ID from the map, returning whether it did anything.
func (m *IDMap) DeleteSnowflake(id discord.Snowflake) bool {
	name := m.Name(id)
	if name == "" {
		// no-op, already deleted
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	name = m.nameHeldRLock(id)
	if name == "" {
		// deleted between rlock release/lock acquire
		return false
	}

	return m.deleteHeldLock(id, name)
}

func (m *IDMap) nameHeldRLock(id discord.Snowflake) string {
	if !id.Valid() {
		panic("Name: invalid ID")
	}

	return m.forward[id]
}

func (m *IDMap) snowflakeHeldRLock(name string) discord.Snowflake {
	if name == "" {
		panic("Snowflake: invalid name")
	}

	return m.backward[name]
}

func (m *IDMap) deleteHeldLock(id discord.Snowflake, name string) bool {
	forwardPresent := m.deleteSnowflakeHeldLock(id, name, false)
	backwardPresent := m.deleteNameHeldLock(id, name, false)

	if forwardPresent != backwardPresent {
		if forwardPresent {
			panic("Delete: ID was present but not name")
		} else {
			panic("Delete: Name was present but not ID")
		}
	}

	return forwardPresent || backwardPresent
}

func (m *IDMap) deleteSnowflakeHeldLock(id discord.Snowflake,
	name string, panicIfMissing bool) bool {
	present := m.nameHeldRLock(id) != ""

	if present {
		delete(m.forward, id)
	} else if panicIfMissing {
		panic("deleteSnowflakeHeldLock: " +
			"tried to delete non-existent snowflake")
	}

	return present
}

func (m *IDMap) deleteNameHeldLock(id discord.Snowflake, name string,
	panicIfMissing bool) bool {
	present := m.snowflakeHeldRLock(name).Valid()

	if present {
		delete(m.backward, name)
	} else if panicIfMissing {
		panic("deleteSnowflakeHeldLock: " +
			"tried to delete non-existent snowflake")
	}

	return present
}

func (m *IDMap) getNewNameHeldRLock(id discord.Snowflake, ideal string) string {
	name := sanitize(ideal)

	for {
		if name == "" || m.snowflakeHeldRLock(name).Valid() {
			name = mangle(name, int64(id))
			continue
		}

		break
	}

	return name
}

func (m *IDMap) setHeldLock(id discord.Snowflake, name string,
	overwriteName, overwriteID bool) {
	m.setNameHeldLock(id, name, overwriteName)
	m.setIDHeldLock(id, name, overwriteID)
}

func (m *IDMap) setNameHeldLock(id discord.Snowflake, name string,
	overwrite bool) {
	if name == "" {
		panic("setNameHeldLock: tried to insert empty string as name")
	}

	if !overwrite && m.nameHeldRLock(id) != "" {
		panic("setIDHeldLock: overwriting without overwrite flag set")
	}

	m.forward[id] = name
}

func (m *IDMap) setIDHeldLock(id discord.Snowflake, name string,
	overwrite bool) {
	if name == "" {
		panic("setIDHeldLock: tried to insert empty string as name")

	}

	if !overwrite && m.snowflakeHeldRLock(name).Valid() {
		panic("setIDHeldLock: overwriting without overwrite flag set")
	}

	m.backward[name] = id
}

// Insert returns an IRC name for a given Discord ID.
// It returns the previous value and the new value.
//
// It returns ideal if there were no collisions.
// It panics if passed an invalid ID.
// Passing an empty string for ideal is allowed, however.
func (m *IDMap) Insert(
	id discord.Snowflake, ideal string) (pre, post string) {
	oldName := m.Name(id)

	if oldName != "" {
		split := strings.SplitN(oldName, "#", 2)
		if split[0] == ideal {
			return oldName, oldName
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	newName := m.getNewNameHeldRLock(id, ideal)

	m.setNameHeldLock(id, newName, true)
	m.setIDHeldLock(id, newName, false)

	if oldName != "" {
		m.deleteNameHeldLock(id, oldName, true)
	}

	return oldName, newName
}

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
