// Package snowflakemap is a bidirectional map from strings to strings.
package snowflakemap

// Some code is similar to https://github.com/vishalkuo/bimap (Copyright (c) 2017 Vishal Kuo), licensed under MIT:
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import (
	"strconv"
	"sync"
)

// SnowflakeMap is a bidirectional map from a string to a discord snowflake (also a string)
type SnowflakeMap struct {
	sync.Mutex
	separator  string
	names      map[string]string // map[snowflake]name
	snowflakes map[string]string // map[name]snowflake
}

// NewSnowflakeMap returns a SnowflakeMap
func NewSnowflakeMap(separator string) *SnowflakeMap {
	return &SnowflakeMap{
		separator:  separator,
		names:      make(map[string]string),
		snowflakes: make(map[string]string),
	}
}

// Add adds a snowflake from a name and snowflake
func (m *SnowflakeMap) Add(name string, snowflake string) string {
	m.Lock()
	defer m.Unlock()
	var suffix string
	for i := 0; ; i++ {
		if i == 0 {
			suffix = ""
		} else {
			suffix = m.separator + strconv.Itoa(i)
		}
		_name := name + suffix

		_, nameExists := m.snowflakes[_name]
		if nameExists {
			continue
		}

		m.snowflakes[_name] = snowflake
		m.names[snowflake] = _name
		return _name
	}
}

// GetName returns a name from a snowflake
func (m *SnowflakeMap) GetName(snowflake string) string {
	m.Lock()
	defer m.Unlock()
	snowflake, exists := m.names[snowflake]
	if exists {
		return snowflake
	}
	return ""
}

// GetNameMap returns a map of names to snowflakes
func (m *SnowflakeMap) GetNameMap() map[string]string {
	m.Lock()
	defer m.Unlock()
	return m.names
}

// GetSnowflake returns a snowflake from a name
func (m *SnowflakeMap) GetSnowflake(name string) string {
	m.Lock()
	defer m.Unlock()
	name, exists := m.snowflakes[name]
	if exists {
		return name
	}
	return ""
}

// GetSnowflakeMap returns a map of snowflakes to names
func (m *SnowflakeMap) GetSnowflakeMap() map[string]string {
	m.Lock()
	defer m.Unlock()
	return m.snowflakes
}

// RemoveName removes an entry corresponding to a name
func (m *SnowflakeMap) RemoveName(name string) {
	m.Lock()
	defer m.Unlock()
	if snowflake, exists := m.snowflakes[name]; exists {
		delete(m.names, snowflake)
		delete(m.snowflakes, name)
	}
}

// RemoveSnowflake removes an entry corresponding to a snowflake
func (m *SnowflakeMap) RemoveSnowflake(snowflake string) {
	m.Lock()
	defer m.Unlock()
	if name, exists := m.names[snowflake]; exists {
		delete(m.names, snowflake)
		delete(m.snowflakes, name)
	}
}

// Length returns the length of the SnowflakeMap
func (m *SnowflakeMap) Length() int {
	m.Lock()
	defer m.Unlock()
	return len(m.names)
}
