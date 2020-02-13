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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	tg "sot-te.ch/TGHelper"
	"strings"
	"time"
)

type ExtractAction struct {
	Action string `json:"action"`
	Param  string `json:"param"`
}

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		URL          string
		Delay        uint   `json:"delay"`
		Threshold    uint   `json:"threshold"`
		IgnoreRegexp string `json:"ignoreregexp"`
	} `json:"crawler"`
	FilesPath string   `json:"filespath"`
	DB        Database `json:"db"`
	Telegram  struct {
		Token    string `json:"token"`
		OTPSeed  string `json:"otpseed"`
		Messages struct {
			tg.TGMessages
			State   string `json:"state"`
			KUpload string `json:"kupload"`
			TUpload string `json:"tupload"`
		} `json:"msg"`
	} `json:"telegram"`
	Kaltura Kaltura `json:"kaltura"`
}

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) getState(chat int64) (string, error) {
	var err error
	var isMob, isAdmin bool
	var pending, converting []TorrentFile
	if isMob, err = cr.DB.GetChatExist(chat); err != nil{
		return "", err
	}
	if isAdmin, err = cr.DB.GetAdminExist(chat); err != nil{
		return "", err
	}
	pendingSB, convertingSB := strings.Builder{}, strings.Builder{}
	if strings.Index(cr.Telegram.Messages.State, msgFilesPending) >= 0 {
		if pending, err = cr.DB.GetTorrentFilesPending(); err != nil{
			return "", err
		}
		if pending != nil {
			for _, val := range pending {
				pendingSB.WriteString(val.String())
				pendingSB.WriteRune('\n')
			}
		}
	}
	if strings.Index(cr.Telegram.Messages.State, msgFilesConverting) >= 0 {
		if converting, err = cr.DB.GetTorrentFilesConverting(); err != nil{
			return "", err
		}
		if converting != nil {
			for _, val := range converting {
				convertingSB.WriteString(val.String())
				convertingSB.WriteRune('\n')
			}
		}
	}
	return formatMessage(cr.Telegram.Messages.State, map[string]interface{}{
		msgWatch: isMob,
		msgAdmin: isAdmin,
		msgFilesPending: pendingSB.String(),
		msgFilesConverting: convertingSB.String(),
	}), err
}

func (cr *Observer) initTg() *tg.Telegram {
	telegram := tg.New(cr.Telegram.OTPSeed)
	telegram.Messages = cr.Telegram.Messages.TGMessages
	telegram.BackendFunctions = tg.TGBackendFunction{
		GetOffset:  cr.DB.GetTgOffset,
		SetOffset:  cr.DB.UpdateTgOffset,
		ChatExist:  cr.DB.GetChatExist,
		ChatAdd:    cr.DB.AddChat,
		ChatRm:     cr.DB.DelChat,
		AdminExist: cr.DB.GetAdminExist,
		AdminAdd:   cr.DB.AddAdmin,
		AdminRm:    cr.DB.DelAdmin,
		State:      cr.getState,
	}
	return telegram
}

func (cr *Observer) Engage() {
	ignorePattern := regexp.MustCompile(cr.Crawler.IgnoreRegexp)
	err := cr.DB.Connect()
	if err == nil {
		defer cr.DB.Close()
		telegram := cr.initTg()
		err = telegram.Connect(cr.Telegram.Token, -1)
		if err == nil {
			go telegram.HandleUpdates()
			baseOffset, err := cr.DB.GetCrawlOffset()
			if err != nil {
				Logger.Error(err)
			}
			for {
				torrents := make([]*Torrent, 0, cr.Crawler.Threshold)
				for i, offset := uint(0), baseOffset; i < cr.Crawler.Threshold; i++ {
					var torrent *Torrent
					baseOffset, torrent = cr.checkTorrent(offset + i, ignorePattern)
					if torrent != nil {
						torrents = append(torrents, torrent)
					}
				}
				if len(torrents) > 0 {
					//TODO: upload to transmission
				}
				//TODO: make session in async function + add telegram upload
				if files, err := cr.DB.GetTorrentFilesPending(); err == nil {
					if files != nil && len(files) > 0 {
						filesToSend := cr.getReadyFiles(files)
						if len(filesToSend) > 0 {
							sort.Strings(filesToSend)
							go cr.Kaltura.UploadFiles(filesToSend)
						}
					}
				} else {
					Logger.Error(err)
				}
				sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
				Logger.Debugf("Sleeping %d sec", sleepTime)
				time.Sleep(sleepTime * time.Second)
			}
		}
	}
	if err != nil {
		Logger.Fatal(err)
	}
}

func (cr *Observer) checkTorrent(currentOffset uint, ignorePattern *regexp.Regexp) (uint, *Torrent) {
	var err error
	var torrent *Torrent
	Logger.Debugf("Checking offset %d", currentOffset)
	fullUrl := fmt.Sprintf(cr.Crawler.URL, currentOffset)
	if torrent, err = GetTorrent(fullUrl); err == nil {
		if torrent != nil {
			Logger.Infof("New file %s", torrent.Info.Name)
			size := torrent.FullSize()
			Logger.Infof("New torrent size %d", size)
			if size > 0 {
				if !ignorePattern.MatchString(torrent.Info.Name) {
					files := torrent.Files()
					Logger.Debugf("Adding torrent %s", torrent.Info.Name)
					Logger.Debugf("Files: %v", files)
					if err = cr.DB.AddTorrent(torrent.Info.Name, files); err != nil {
						Logger.Error(err)
					}
				} else {
					Logger.Infof("Torrent %s ignored", torrent.Info.Name)
				}
				currentOffset++
				if err = cr.DB.UpdateCrawlOffset(currentOffset); err != nil {
					Logger.Error(err)
				}
			} else {
				Logger.Errorf("Zero torrent size, offset %d", currentOffset)
			}
		} else {
			Logger.Debugf("%s not a torrent", fullUrl)
		}
	}
	return currentOffset, torrent
}

func (cr *Observer) getReadyFiles(files []TorrentFile) []string {
	var filesToSend []string
	for _, file := range files {
		fullPath := filepath.Join(cr.FilesPath, file.Name)
		fullPath = filepath.FromSlash(fullPath)
		if stat, err := os.Stat(fullPath); err == nil {
			if stat == nil {
				Logger.Warningf("Unable to stat file %s", fullPath)
			} else {
				Logger.Debugf("Found ready file %s, size: %d", stat.Name(), stat.Size())
				filesToSend = append(filesToSend, fullPath)
				if err := cr.DB.SetTorrentFileConverting(file.Id); err != nil {
					Logger.Error(err)
				}
			}
		}
	}
	return filesToSend
}
