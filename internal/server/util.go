package server

import (
	"fmt"

	"gopkg.in/irc.v3"
)

func checkParamCount(msg *irc.Message, minParamCount, maxParamCount int) error {
	params := len(msg.Params)
	if params < minParamCount {
		return fmt.Errorf(
			"Not enough parameters for %s "+
				"(expected at least %d, got %d)",
			msg.Command, minParamCount, params)
	}
	if maxParamCount != -1 && params > maxParamCount {
		return fmt.Errorf(
			"Too many parameters for %s "+
				"(expected at most %d, got %d)",
			msg.Command, maxParamCount, params)
	}
	return nil
}
