package ircdiscord

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/src/session"
	"gopkg.in/irc.v3"
)

type Client struct {
	conn               net.Conn
	irc                *irc.Conn
	session            *session.Session
	guild              *discord.Guild
	subscribedChannels map[discord.Snowflake]string
	serverPrefix       irc.Prefix
	clientPrefix       irc.Prefix
	lastMessageID      discord.Snowflake // the ID of the last message this client sent
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:               conn,
		irc:                irc.NewConn(conn),
		subscribedChannels: make(map[discord.Snowflake]string),
	}
}

func (c *Client) Close() error {
	if c.session != nil {
		c.session.Unref()
	}
	return c.conn.Close()
}

func (c *Client) Run() error {
	defer c.Close()

	c.serverPrefix.Name = c.conn.LocalAddr().String()
	c.clientPrefix.Name = c.conn.RemoteAddr().String()

	log.Printf("connected: %v", c.clientPrefix.Name)
	defer log.Printf("disconnected: %v", c.clientPrefix.Name)

initial_loop:
	for {
		msg, err := c.irc.ReadMessage()
		if err != nil {
			return err
		}
		switch msg.Command {
		case "CAP", "NICK", "USER":
			// intentionally left blank
		case "PASS":
			if len(msg.Params) != 1 {
				return fmt.Errorf("invalid parameter count for PASS")
			}
			args := strings.SplitN(msg.Params[0], ":", 2)
			session, err := session.Get(args[0])
			if err != nil {
				return err
			}
			c.session = session
			if len(args) > 1 {
				snowflake, err := discord.ParseSnowflake(args[1])
				if err != nil {
					return err
				}
				guild, err := c.session.Guild(snowflake)
				if err != nil {
					return err
				}
				c.session.Gateway.GuildSubscribe(gateway.GuildSubscribeData{
					GuildID: guild.ID,
				})
				c.guild = guild
			}
			break initial_loop
		default:
			return fmt.Errorf("invalid command received in authentication stage: %v",
				msg.Command)
		}
	}

	me, err := c.session.Me()
	if err != nil {
		return err
	}

	c.clientPrefix = *discordUserPrefix(me)

	if err := c.sendGreeting(); err != nil {
		return err
	}

	msgs := make(chan *irc.Message)
	errors := make(chan error)

	go func() {
		for {
			msg, err := c.irc.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			msgs <- msg
		}
	}()

	events, cancel := c.session.ChanFor(func(e interface{}) bool { return true })
	defer cancel()

	for {
		select {
		case msg := <-msgs:
			if err := c.handleIRCMessage(msg); err != nil {
				return err
			}
		case event := <-events:
			if err := c.handleDiscordEvent(event); err != nil {
				return err
			}
		case err := <-errors:
			return err
		}
	}
}

func (c *Client) sendGreeting() error {
	me, err := c.session.Me()
	if err != nil {
		return err
	}

	guildName := "Discord"
	guildID := me.ID
	if c.guild != nil {
		guildName = c.guild.Name
		guildID = c.guild.ID
	}

	if err := c.irc.WriteMessage(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.RPL_WELCOME,
		Params: []string{c.clientPrefix.Name, fmt.Sprintf("Welcome to %s, %s#%s",
			guildName, me.Username, me.Discriminator)},
	}); err != nil {
		return err
	}

	if err := c.irc.WriteMessage(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.RPL_YOURHOST,
		Params: []string{c.clientPrefix.Name,
			fmt.Sprintf("Your host is %s, running ircdiscord", c.serverPrefix.Name)},
	}); err != nil {
		return err
	}

	if err := c.irc.WriteMessage(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.RPL_CREATED,
		Params: []string{c.clientPrefix.Name,
			fmt.Sprintf("This server was created %s", guildID.Time().String())},
	}); err != nil {
		return err
	}

	return nil
}
