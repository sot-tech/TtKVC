/*
 * BSD-3-Clause
 * Copyright 2020 sot (PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package TtKVC

import (
	tlg "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xlzd/gotp"
	"strconv"
	"strings"
	"time"
)

const (
	cmdStart    = "start"
	cmdAttach   = "attach"
	cmdDetach   = "detach"
	cmdState    = "state"
)

type Telegram struct {
	Bot      *tlg.BotAPI
	DB       *Database
	TOTP     *gotp.TOTP
}

func (tg *Telegram) processCommand(msg *tlg.Message) {
	chat := msg.Chat.ID
	resp := Messages.Commands.Unknown
	cmd := msg.Command()
	switch cmd {
	case cmdStart:
		resp = strings.Replace(Messages.Commands.Start, msgVersion, Version, -1)
	case cmdAttach:
		if tg.TOTP.Verify(msg.CommandArguments(), int(time.Now().Unix())) {
			if err := tg.DB.AddChat(chat); err == nil {
				Logger.Noticef("New chat added %d", chat)
				resp = Messages.Commands.Attach
			} else {
				Logger.Warningf("Attach: %v", err)
				resp = strings.Replace(Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			Logger.Infof("Attach unauthorized %d", chat)
			resp = Messages.Commands.Unauthorized
		}
	case cmdDetach:
		if chatExist, err := tg.DB.GetChatExist(chat); chatExist {
			if err := tg.DB.DelChat(chat); err == nil {
				Logger.Noticef("Chat deleted %d", chat)
				resp = Messages.Commands.Detach
			} else {
				Logger.Warningf("Attach: %v", err)
				resp = strings.Replace(Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			if err == nil {
				Logger.Infof("Detach unauthorized %d", chat)
				resp = Messages.Commands.Unauthorized
			} else {
				Logger.Warningf("Detach: %v", err)
				resp = strings.Replace(Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		}
	case cmdState:
		if chatExist, err := tg.DB.GetChatExist(chat); chatExist {
			var err error
			var offset uint
			var files []TorrentFile
			offset, err = tg.DB.GetCrawlOffset()
			files, err = tg.DB.GetTorrentFilesNotReady()
			if err == nil {
				resp = Messages.Commands.State
				resp = strings.Replace(resp, msgIndex, strconv.FormatUint(uint64(offset), 10), -1)
				resp = strings.Replace(resp, msgVersion, Version, -1)
				if strings.Index(resp, msgFiles) >= 0 {
					sb := strings.Builder{}
					if files != nil && len(files) > 0 {
						for _, v := range files {
							sb.WriteString(v.Name)
							sb.WriteRune('\n')
						}
					}
					resp = strings.Replace(resp, msgFiles, sb.String(), -1)
				}
			} else {
				Logger.Warningf("State: %v", err)
				resp = strings.Replace(Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			if err == nil {
				Logger.Infof("State unauthorized %d", chat)
				resp = Messages.Commands.Unauthorized
			} else {
				Logger.Warningf("State: %v", err)
				resp = strings.Replace(Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		}
	}
	tg.sendMsg(resp, []int64{chat}, false)
}

func (tg *Telegram) HandleUpdates() {
	offset, err := tg.DB.GetTgOffset()
	if err != nil {
		Logger.Error(err)
	}
	updateConfig := tlg.NewUpdate(offset)
	updateConfig.Timeout = 60
	if updateChannel, err := tg.Bot.GetUpdatesChan(updateConfig); err == nil {
		for up := range updateChannel {
			Logger.Noticef("Got new update: %v", up)
			var msg *tlg.Message
			if up.Message != nil && up.Message.IsCommand() {
				msg = up.Message
			} else if up.ChannelPost != nil && up.ChannelPost.IsCommand() {
				msg = up.ChannelPost
			}
			if msg != nil {
				go tg.processCommand(msg)
			}
			if up.UpdateID > offset {
				offset = up.UpdateID
				if err = tg.DB.UpdateTgOffset(offset); err != nil {
					Logger.Error(err)
				}
			}
		}
	} else {
		Logger.Errorf("Unable to get telegram update channel, commands disabled: %v", err)
	}
}

func (tg *Telegram) sendMsg(msgText string, chats []int64, formatted bool) {
	if msgText != "" && chats != nil && len(chats) > 0 {
		Logger.Debugf("Sending message %s to %v", msgText, chats)
		var photoId string
		for _, chat := range chats {
			msg := tlg.NewMessage(chat, msgText)
			if formatted {
				msg.ParseMode = "Markdown"
			}
			if sentMsg, err := tg.Bot.Send(msg); err == nil {
				Logger.Debugf("Message to %d has been sent", chat)
				if photoId == "" && sentMsg.Photo != nil && len(*sentMsg.Photo) > 0 {
					photoId = (*sentMsg.Photo)[0].FileID
				}
			} else {
				Logger.Error(err)
			}
		}
	}
}

func (tg *Telegram) SendMsgToAll(msg string) {
	var chats []int64
	var err error
	if chats, err = tg.DB.GetChats(); err != nil {
		Logger.Error(err)
	}
	tg.sendMsg(msg, chats, true)
}

func (tg *Telegram) Connect(token, otpSeed string, tries int) error {
	var err error
	tg.TOTP = gotp.NewDefaultTOTP(otpSeed)
	for try := 0; try < tries || tries < 0; try++ {
		if tg.Bot, err = tlg.NewBotAPI(token); err == nil {
			tg.Bot.Debug = false
			Logger.Infof("Authorized on account %s", tg.Bot.Self.UserName)
			break
		} else {
			Logger.Errorf("Unable to connect to telegram, try %d of %d: %s\n", try, tries, err)
			time.Sleep(10 * time.Second)
		}
	}
	return err
}
