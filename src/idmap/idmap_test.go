package idmap

import "testing"

func assert(cond bool) {
	if !cond {
		panic("assertion failed")
	}
}

func TestMangle(t *testing.T) {
	var res string
	res = mangle("name", 12345)
	assert(res == "name#1")
	res = mangle("name#1", 12345)
	assert(res == "name#12")
	res = mangle("name#12", 12345)
	assert(res == "name#123")
	res = mangle("name#123", 12345)
	assert(res == "name#1234")
	res = mangle("name#1234", 12345)
	assert(res == "name#12345")
	res = mangle("name#12345", 12345)
	assert(res == "name#12345#")
	res = mangle("name#12345#", 12345)
	assert(res == "name#12345##")
}
