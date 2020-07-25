package ilayer

import (
	"fmt"
	"strings"

	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"gopkg.in/irc.v3"
)

var supportedCapabilities = map[string]string{
	"echo-message":      "",
	"server-time":       "",
	"message-tags":      "",
	"sasl":              "PLAIN",
	"batch":             "",
	"draft/chathistory": "",
}

func (c *Client) handleCap(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, -1); err != nil {
		return err
	}

	switch msg.Params[0] {
	case "LS":
		return c.handleCapLs(msg)
	case "LIST":
		return c.handleCapList(msg)
	case "REQ":
		return c.handleCapReq(msg)
	case "END":
		return c.handleCapEnd(msg)
	default:
		return nil
	}
}

func (c *Client) handleCapLs(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 2); err != nil {
		return err
	}

	if err := replies.CAP_LS(c, supportedCapabilities); err != nil {
		return err
	}

	c.isCapBlocked = true

	return nil
}

func (c *Client) handleCapList(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 2); err != nil {
		return err
	}

	enabledcaps := []string{}
	for capability := range c.capabilities {
		enabledcaps = append(enabledcaps, capability)

	}

	return replies.CAP_LIST(c, enabledcaps)
}

func (c *Client) handleCapReq(msg *irc.Message) error {
	if err := checkParamCount(msg, 2, 2); err != nil {
		return err
	}

	requested := strings.Split(msg.Params[1], " ")
	for _, capability := range requested {
		supported := false

		for suppcap, _ := range supportedCapabilities {
			if capability == suppcap {
				supported = true
				break
			}
		}

		if !supported {
			return fmt.Errorf(
				"unknown capability requested: %s",
				capability)
		}

		c.capabilities[capability] = struct{}{}
	}

	if err := replies.CAP_ACK(c, requested); err != nil {
		return err
	}

	c.isCapBlocked = true

	return nil
}

func (c *Client) handleCapEnd(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	c.isCapBlocked = false

	return c.maybeCompleteRegistration()
}
