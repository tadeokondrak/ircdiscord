package main

func truncate(str string, chars int) string {
	if len(str) >= chars {
		return str[0:chars]
	}
	return str
}
