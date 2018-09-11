package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// -----------------------------------------------------------------------------

func (bot *Bot) err(cmd []string, post string, err error) {
	println("CMD:ERROR " + err.Error())
	bot.SendMsgToChannel(fmt.Sprintf("---\n:red_circle: cmd %s ERROR: %+v", cmd[0], err), post)
}

func (bot *Bot) run(cmd []string, post string) error {
	bot.SendMsgToChannel("---", post)
	shell := exec.Command(bot.config.Command, cmd...)
	stdout, err := shell.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := shell.StderrPipe()
	if err != nil {
		return err
	}
	wg := new(sync.WaitGroup)
	go readChan(wg, stdout, bot, post, "OUT>", "")
	go readChan(wg, stderr, bot, post, "ERR>", ":exclamation: ")

	if err := shell.Start(); err != nil {
		return err
	}

	err = shell.Wait()
	wg.Wait()
	if err != nil {
		return err
	}
	println("CMD:SUCCESS")
	bot.SendMsgToChannel(fmt.Sprintf("---\n:white_check_mark: cmd %s executed", cmd[0]), post)
	return nil
}

func readChan(wg *sync.WaitGroup, ch io.ReadCloser, bot *Bot, post, prefix, md string) {
	wg.Add(1)
	defer wg.Done()
	reader := bufio.NewReader(ch)

	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}
		//print(prefix + str)
		bot.SendMsgToChannel(md+str, post)
	}
}
