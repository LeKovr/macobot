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

	"github.com/jessevdk/go-flags"
	"github.com/mattermost/mattermost-server/model"
)

// Config holds all config vars
type Config struct {
	Addr     string `long:"addr" 			description:"Mattermost server address"`
	Login    string `long:"login" 		description:"Bot login"`
	Password string `long:"password" 	description:"Bot password"`
	Team     string `long:"team"  		description:"Bot team"`
	Channel  string `long:"channel" 	description:"Command channel"`
	Command  string `long:"command" 	description:"Command file" default:"macobot.sh"`
}

const ()

var client *model.Client4
var webSocketClient *model.WebSocketClient

var botUser *model.User
var botTeam *model.Team
var debuggingChannel *model.Channel

// Documentation for the Go driver can be found
// at https://godoc.org/github.com/mattermost/platform/model#Client
func main() {

	var cfg Config
	_, err := flags.Parse(&cfg)
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(1) // help printed
		} else {
			os.Exit(2) // error message written already
		}
	}

	println(cfg.Login)

	SetupGracefulShutdown()

	client = model.NewAPIv4Client(cfg.Addr)

	// Lets test to see if the mattermost server is up and running
	MakeSureServerIsRunning()

	// lets attempt to login to the Mattermost server as the bot user
	// This will set the token required for all future calls
	// You can get this token with client.AuthToken
	LoginAsTheBotUser(&cfg)

	// Lets find our bot team
	FindBotTeam(&cfg)

	// This is an important step.  Lets make sure we use the botTeam
	// for all future web service requests that require a team.
	//client.SetTeamId(botTeam.Id)

	// Lets create a bot channel for logging debug messages into
	AttachBotDebuggingChannel(&cfg)

	// Lets start listening to some channels via the websocket!
	wsaddr := strings.Replace(cfg.Addr, "http", "ws", 1)
	webSocketClient, err := model.NewWebSocketClient4(wsaddr, client.AuthToken)
	if err != nil {
		println("We failed to connect to the web socket " + wsaddr)
		PrintErrorT(err)
		os.Exit(1)

	}

	webSocketClient.Listen()
	SendMsgToDebuggingChannel("_"+botUser.Username+" has **started** running_", "")

	go func() {
		for {
			select {
			case resp := <-webSocketClient.EventChannel:
				HandleWebSocketResponse(resp)
			}
		}
	}()

	// You can block forever with
	select {}
}

func MakeSureServerIsRunning() {
	if props, resp := client.GetOldClientConfig(""); resp.Error != nil {
		println("There was a problem pinging the Mattermost server.  Are you sure it's running?")
		PrintErrorM(resp.Error)
		os.Exit(1)
	} else {
		println("Server detected and is running version " + props["Version"])
	}
}

func LoginAsTheBotUser(cfg *Config) {
	if user, resp := client.Login(cfg.Login, cfg.Password); resp.Error != nil {
		println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botUser = user
	}
}

func FindBotTeam(cfg *Config) {
	if team, resp := client.GetTeamByName(cfg.Team, ""); resp.Error != nil {
		println("We failed to get the initial load")
		println("or we do not appear to be a member of the team '" + cfg.Team + "'")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botTeam = team
	}
}

func AttachBotDebuggingChannel(cfg *Config) {
	if rchannel, resp := client.GetChannelByName(cfg.Channel, botTeam.Id, ""); resp.Error != nil {
		println("We failed to get the channels")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		debuggingChannel = rchannel
		return
	}

}

func SendMsgToDebuggingChannel(msg string, replyToId string) {
	post := &model.Post{}
	post.ChannelId = debuggingChannel.Id
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	HandleMsgFromDebuggingChannel(event)
}

func HandleMsgFromDebuggingChannel(event *model.WebSocketEvent) {
	// If this isn't the debugging channel then lets ingore it
	if event.Broadcast.ChannelId != debuggingChannel.Id {
		return
	}

	// Lets only reponded to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	println("responding to debugging channel msg")

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		/*
			// ignore my events
			if post.UserId == botUser.Id {
				return
			}
		*/
		// if you see any word matching 'alive' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)alive(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'up' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)up(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'running' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)running(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'hello' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)hello(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}
	}

	SendMsgToDebuggingChannel("I did not understand you!", post.Id)
}

func PrintError(err error) { //*model.AppError) {
	println("\t%v", err)
}

func PrintErrorT(err interface{}) { //*model.AppError) {
	fmt.Printf("\t%+v", err)
}

func PrintErrorM(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
}

func SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			if webSocketClient != nil {
				webSocketClient.Close()
			}

			SendMsgToDebuggingChannel("**stopped** running_", "")
			os.Exit(0)
		}
	}()
}
