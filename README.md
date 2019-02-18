# IRCdiscord

[![CircleCI](https://circleci.com/gh/tadeokondrak/IRCdiscord/tree/master.svg?style=svg)](https://circleci.com/gh/tadeokondrak/IRCdiscord/tree/master) [![Discord](https://img.shields.io/discord/541500289430192150.svg?logo=discord&style=flat-square)](https://discord.gg/TeJbfad)

An IRCd that lets you talk to Discord users. It essentially is an IRC server that connects to Discord to relay messages between your IRC client and Discord.

# Capabilities
Listed below are current features.
-/whois gives information on discord users
-DM support
-Talk in any server/channel
-/list lists all channels in server

# Installation
Build with `go build` and then copy into your $PATH. You can also grab a prebuilt binary above.

# Usage
Run the program, and in your IRC client, connect to `127.0.0.1` with the server password being `<your discord token>:<target discord server id>`.

An example for weechat:
```
/server add discordserver -password=lkajf_343jlksaf43wjalfkjdsaf:348734324
/set irc.server.discordserver.capabilities "server-time"
/set irc.server.discordserver.autoconnect on
/set irc.server.discordserver.autojoin "#channel1,#channel2,#channel3"
/connect discordserver
```
If the server ID is omitted, then it will join a server with no channels but with DM capabilities.

# License
ISC; see LICENSE file. 
