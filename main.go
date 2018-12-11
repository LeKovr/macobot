// Copyright (c) 2016 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

// This code based on https://github.com/mattermost/mattermost-bot-sample-golang/blob/master/bot_sample.go

package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/mattermost/mattermost-server/model"
)

// Config holds all config vars
type Config struct {
	Addr        string `long:"addr"       description:"Mattermost server address"`
	Login       string `long:"login"      description:"Bot login"`
	Password    string `long:"password"   description:"Bot password"`
	Team        string `long:"team"       description:"Bot team"`
	Channel     string `long:"channel"    description:"Command channel"`
	Command     string `long:"command"    description:"Command file" default:"./macobot.sh"`
	CommandWait int    `long:"wait"       description:"Wait command exec (secs)" default:"5"`
	IssueLink   string `long:"issue_link" description:"Format #NNN with this link"`
}

var (
	reIDs = regexp.MustCompile(`(?:^#|\s#)(\d+)($|\W)`)
	reCmd = regexp.MustCompile(`(?:^=)(\w+)((?:\s+)(.+))?$`)

	version = "0.1-dev"
)

type Bot struct {
	config  *Config
	client  *model.Client4
	user    *model.User
	team    *model.Team
	channel *model.Channel
	started time.Time
}

// Documentation for the Go driver can be found
// at https://godoc.org/github.com/mattermost/platform/model#Client
func main() {

	// Parse command args
	cfg := ParseFlags()

	bot := &Bot{config: cfg, started: time.Now()}

	bot.client = model.NewAPIv4Client(cfg.Addr)

	// Lets test to see if the mattermost server is up and running
	bot.MakeSureServerIsRunning()

	// lets attempt to login to the Mattermost server as the bot user
	// This will set the token required for all future calls
	// You can get this token with client.AuthToken
	bot.LoginAsTheBotUser()
	println("Logged in as " + bot.user.Username)

	// Lets find our bot team
	bot.FindBotTeam()

	// Attach bot to existing channel
	bot.AttachBotChannel()

	// Lets start listening to some channels via the websocket!
	wsaddr := strings.Replace(cfg.Addr, "http", "ws", 1)
	webSocketClient, err := model.NewWebSocketClient4(wsaddr, bot.client.AuthToken)
	if err != nil {
		println("We failed to connect to the web socket " + wsaddr)
		PrintErrorM(err)
		os.Exit(1)
	}

	webSocketClient.Listen()
	bot.SendMsgToChannel("_"+bot.user.Username+" **connected**_", "")
	go func() {
		for {
			select {
			case resp := <-webSocketClient.EventChannel:
				bot.HandleWebSocketResponse(resp)
			}
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	bot.SendMsgToChannel("_"+bot.user.Username+" **disconnected**_", "")
	if webSocketClient != nil {
		webSocketClient.Close()
	}
	println("Shutdown")
}

func ParseFlags() *Config {
	cfg := Config{}
	_, err := flags.Parse(&cfg)
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(1) // help printed
		} else {
			println(err.Error())
			os.Exit(2) // error message written already
		}
	}
	return &cfg
}

func (bot *Bot) MakeSureServerIsRunning() {
	if props, resp := bot.client.GetOldClientConfig(""); resp.Error != nil {
		println("There was a problem pinging the Mattermost server.  Are you sure it's running?")
		PrintErrorM(resp.Error)
		os.Exit(1)
	} else {
		println("Server detected and is running version " + props["Version"])
	}
}

func (bot *Bot) LoginAsTheBotUser() {
	if user, resp := bot.client.Login(bot.config.Login, bot.config.Password); resp.Error != nil {
		println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		bot.user = user
	}
}

func (bot *Bot) FindBotTeam() {
	if team, resp := bot.client.GetTeamByName(bot.config.Team, ""); resp.Error != nil {
		println("We failed to get the initial load")
		println("or we do not appear to be a member of the team '" + bot.config.Team + "'")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		bot.team = team
	}
}

func (bot *Bot) AttachBotChannel() {
	if rchannel, resp := bot.client.GetChannelByName(bot.config.Channel, bot.team.Id, ""); resp.Error != nil {
		println("We failed to get the channels")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		bot.channel = rchannel
		return
	}
}

func (bot *Bot) SendMsgToChannelByID(msg string, replyToId string, channelID string) {
	post := &model.Post{}
	post.ChannelId = channelID
	post.Message = msg

	if _, resp := bot.client.CreatePost(post); resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintErrorM(resp.Error)
	}
}

func (bot *Bot) SendMsgToChannel(msg string, replyToId string) {
	bot.SendMsgToChannelByID(msg, replyToId, bot.channel.Id)
}

func (bot *Bot) HandleWebSocketResponse(event *model.WebSocketEvent) {
	bot.HandleMsgFromChannel(event)
}

func (bot *Bot) HandleMsgFromChannel(event *model.WebSocketEvent) {
	// Lets only reponded to messaged posted events
	if event == nil || event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		// ignore my events
		if post.UserId == bot.user.Id {
			return
		}

		// Check if post contains issue id(s)
		if bot.config.IssueLink != "" {
			if matched := reIDs.FindAllStringSubmatch(post.Message, -1); len(matched) > 0 {
				s := ""
				for _, v := range matched {
					id := v[1]
					s += fmt.Sprintf(bot.config.IssueLink, id)
				}
				if s != "" {
					bot.SendMsgToChannelByID("Post links:\n"+s, post.Id, post.ChannelId)
				}
			}
		}

		// If this isn't the debugging channel then lets ingore it
		if event.Broadcast.ChannelId != bot.channel.Id {
			return
		}

		// Check if post contains bot command
		if matches := reCmd.FindStringSubmatch(post.Message); len(matches) > 0 {
			println("GOT> " + post.Message)
			if matches[1] == "uptime" {
				bot.SendMsgToChannel("I'm up since "+bot.started.Format("Mon Jan _2 15:04:05 2006"), post.Id)
			} else {
				user, _ := bot.client.GetUser(post.UserId, "")
				bot.SendMsgToChannel(fmt.Sprintf("Выполняю команду **%s** по запросу @%s", matches[1], user.Username), post.Id)
				err := bot.run(matches[1:], post.Id)
				if err != nil {
					bot.err(matches[1:], post.Id, err)
				}
			}
			return
		}

	}
}

func PrintError(err error) {
	fmt.Printf("Error: %+v", err)
}

func PrintErrorM(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
}
