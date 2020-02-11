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
	"sot-te.ch/TtKVCv0/intl"
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
	Kaltura       intl.Kaltura `json:"kaltura"`
	FilesPath     string        `json:"filespath"`
	TelegramToken string        `json:"telegramtoken"`
	AdminOTPSeed  string        `json:"adminotpseed"`
	DBFile        string        `json:"dbfile"`
	Messages      intl.Message `json:"msg"`
}

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) Engage() {
	database := &intl.Database{}
	intl.Messages = cr.Messages
	telegram := &intl.Telegram{
		DB:       database,
	}
	cr.Kaltura.Telegram = telegram
	ignorePattern := regexp.MustCompile(cr.Crawler.IgnoreRegexp)
	err := database.Connect(cr.DBFile)
	if err == nil {
		defer database.Close()
		err = telegram.Connect(cr.TelegramToken, cr.AdminOTPSeed, -1)
		if err == nil {
			go telegram.HandleUpdates()
			baseOffset, err := database.GetCrawlOffset()
			if err != nil {
				intl.Logger.Error(err)
			}
			for {
				var i, offset uint
				offset = baseOffset
				for i = 0; i < cr.Crawler.Threshold; i++ {
					baseOffset = checkTorrent(cr.Crawler.URL, offset, baseOffset, i, ignorePattern, database)
				}
				if files, err := database.GetTorrentFilesNotReady(); err == nil {
					if files != nil && len(files) > 0 {
						filesToSend := getReadyFiles(database, files, cr.FilesPath)
						if len(filesToSend) > 0 {
							sort.Strings(filesToSend)
							go cr.Kaltura.ProcessFiles(filesToSend)
						}
					}
				} else {
					intl.Logger.Error(err)
				}
				sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
				intl.Logger.Debugf("Sleeping %d sec", sleepTime)
				time.Sleep(sleepTime * time.Second)
			}
		}
	}
	if err != nil {
		intl.Logger.Fatal(err)
	}
}

func checkTorrent(baseUrl string, offset, baseOffset, i uint, ignorePattern *regexp.Regexp, database *intl.Database) uint {
	currentOffset := offset + i
	intl.Logger.Debugf("Checking offset %d", currentOffset)
	fullUrl := fmt.Sprintf(baseUrl, currentOffset)
	if torrent, err := intl.GetTorrent(fullUrl); err == nil {
		if torrent != nil {
			intl.Logger.Infof("New file %s", torrent.Info.Name)
			size := torrent.FullSize()
			intl.Logger.Infof("New torrent size %d", size)
			if size > 0 {
				if !ignorePattern.MatchString(torrent.Info.Name) {
					files := torrent.Files()
					intl.Logger.Debugf("Adding torrent %s", torrent.Info.Name)
					intl.Logger.Debugf("Files: %v", files)
					if err := database.AddTorrent(torrent.Info.Name, files); err != nil {
						intl.Logger.Error(err)
					}
				} else{
					intl.Logger.Infof("Torrent %s ignored", torrent.Info.Name)
				}
				baseOffset = currentOffset + 1
				if err := database.UpdateCrawlOffset(baseOffset); err != nil {
					intl.Logger.Error(err)
				}
			} else {
				intl.Logger.Errorf("Zero torrent size, offset %d", currentOffset)
			}
		} else {
			intl.Logger.Debugf("%s not a torrent", fullUrl)
		}
	}
	return baseOffset
}

func getReadyFiles(database *intl.Database, files []intl.TorrentFile, path string) []string{
	var filesToSend []string
	for _, file := range files {
		fullPath := filepath.Join(path, file.Name)
		fullPath = filepath.FromSlash(fullPath)
		if stat, err := os.Stat(fullPath); err == nil {
			if stat == nil {
				intl.Logger.Warningf("Unable to stat file %s", fullPath)
			} else {
				intl.Logger.Debugf("Found ready file %s, size: %d", stat.Name(), stat.Size())
				filesToSend = append(filesToSend, fullPath)
				if err := database.SetTorrentFileReady(file.Id); err != nil {
					intl.Logger.Error(err)
				}
			}
		}
	}
	return filesToSend
}
