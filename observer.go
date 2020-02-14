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
	"encoding/base64"
	"encoding/json"
	"fmt"
	trans "github.com/hekmon/transmissionrpc"
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
	Transmission struct {
		Host       string `json:"host"`
		Port       uint16 `json:"port"`
		Login      string `json:"login"`
		Password   string `json:"password"`
		Path       string `json:"path"`
		Encryption bool   `json:"encryption"`
	} `json:"transmission"`
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
	config := new(Observer)
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, config)
	}
	return config, err
}

func (cr *Observer) getState(chat int64) (string, error) {
	var err error
	var isMob, isAdmin bool
	var pending, converting []*TorrentFile
	if isMob, err = cr.DB.GetChatExist(chat); err != nil {
		return "", err
	}
	if isAdmin, err = cr.DB.GetAdminExist(chat); err != nil {
		return "", err
	}
	pendingSB, convertingSB := strings.Builder{}, strings.Builder{}
	if strings.Index(cr.Telegram.Messages.State, msgFilesPending) >= 0 {
		if pending, err = cr.DB.GetTorrentFilesPending(); err != nil {
			return "", err
		}
		if pending != nil {
			for _, val := range pending {
				if val != nil {
					pendingSB.WriteString(val.String())
					pendingSB.WriteRune('\n')
				}
			}
		}
	}
	if strings.Index(cr.Telegram.Messages.State, msgFilesConverting) >= 0 {
		if converting, err = cr.DB.GetTorrentFilesConverting(); err != nil {
			return "", err
		}
		if converting != nil {
			for _, val := range converting {
				if val != nil {
					convertingSB.WriteString(val.String())
					convertingSB.WriteRune('\n')
				}
			}
		}
	}
	return formatMessage(cr.Telegram.Messages.State, map[string]interface{}{
		msgWatch:           isMob,
		msgAdmin:           isAdmin,
		msgFilesPending:    pendingSB.String(),
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

func (cr *Observer) initTransmission() (*trans.Client, error) {
	port := cr.Transmission.Port
	if port <= 0 {
		port = 9091
	}
	return trans.New(cr.Transmission.Host, cr.Transmission.Login, cr.Transmission.Password, &trans.AdvancedConfig{
		HTTPS: cr.Transmission.Encryption,
		Port:  port,
	})
}

func (cr *Observer) Engage() {
	ignorePattern := regexp.MustCompile(cr.Crawler.IgnoreRegexp)
	err := cr.DB.Connect()
	if err == nil {
		defer cr.DB.Close()
		telegram := cr.initTg()
		err = telegram.Connect(cr.Telegram.Token, -1)
		if err == nil {
			if err != nil {
				logger.Error(err)
			}
			go telegram.HandleUpdates()
			baseOffset, err := cr.DB.GetCrawlOffset()
			if err != nil {
				logger.Error(err)
			}
			for {
				torrents := make([]*Torrent, 0, cr.Crawler.Threshold)
				for i, offset := uint(0), baseOffset; i < cr.Crawler.Threshold; i++ {
					var torrent *Torrent
					baseOffset, torrent = cr.checkTorrent(offset+i, ignorePattern)
					if torrent != nil {
						torrents = append(torrents, torrent)
					}
				}
				if len(torrents) > 0 {
					cr.uploadTorrents(torrents)
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
					logger.Error(err)
				}
				sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
				logger.Debugf("Sleeping %d sec", sleepTime)
				time.Sleep(sleepTime * time.Second)
			}
		}
	}
	if err != nil {
		logger.Fatal(err)
	}
}

func (cr *Observer) checkTorrent(currentOffset uint, ignorePattern *regexp.Regexp) (uint, *Torrent) {
	var err error
	var torrent *Torrent
	logger.Debugf("Checking offset %d", currentOffset)
	fullUrl := fmt.Sprintf(cr.Crawler.URL, currentOffset)
	if torrent, err = GetTorrent(fullUrl); err == nil {
		if torrent != nil {
			logger.Infof("New file %s", torrent.Info.Name)
			size := torrent.FullSize()
			logger.Infof("New torrent size %d", size)
			if size > 0 {
				if !ignorePattern.MatchString(torrent.Info.Name) {
					files := torrent.Files()
					logger.Debugf("Adding torrent %s", torrent.Info.Name)
					logger.Debugf("Files: %v", files)
					if err = cr.DB.AddTorrent(torrent.Info.Name, files); err != nil {
						logger.Error(err)
					}
				} else {
					logger.Infof("Torrent %s ignored", torrent.Info.Name)
				}
				currentOffset++
				if err = cr.DB.UpdateCrawlOffset(currentOffset); err != nil {
					logger.Error(err)
				}
			} else {
				logger.Errorf("Zero torrent size, offset %d", currentOffset)
			}
		} else {
			logger.Debugf("%s not a torrent", fullUrl)
		}
	}
	return currentOffset, torrent
}

func (cr *Observer) uploadTorrents(newTorrents []*Torrent) {
	if transmission, err := cr.initTransmission(); err == nil && transmission != nil {
		if existingTorrents, err := transmission.TorrentGet([]string{"id", "name"}, nil); err == nil {
			if existingTorrents != nil {
				torrentsToRm := make([]int64, 0, len(newTorrents))
				for _, existingTorrent := range existingTorrents {
					if existingTorrent != nil && existingTorrent.Name != nil &&
						existingTorrent.ID != nil {
						for _, newTorrent := range newTorrents {
							if *(existingTorrent.Name) == newTorrent.Info.Name {
								logger.Debugf("Torrent %v marked as toDelete", *(existingTorrent))
								torrentsToRm = append(torrentsToRm, *existingTorrent.ID)
							}
						}
					}
				}
				if len(torrentsToRm) > 0 {
					if err := transmission.TorrentRemove(&trans.TorrentRemovePayload{
						IDs:             torrentsToRm,
						DeleteLocalData: false,
					}); err == nil {
						logger.Debugf("Torrents %v deleted", torrentsToRm)
					} else {
						logger.Error(err)
					}
				}
			}
		} else {
			logger.Error(err)
		}
		falsePtr := new(bool)
		for _, newTorrent := range newTorrents {
			b64 := base64.StdEncoding.EncodeToString(newTorrent.RawData)
			if addedTorrent, err := transmission.TorrentAdd(&trans.TorrentAddPayload{
				DownloadDir: &cr.Transmission.Path,
				MetaInfo:    &b64,
				Paused:      falsePtr,
			}); err == nil {
				if addedTorrent != nil {
					logger.Debugf("Added torrent")
				} else {
					logger.Warningf("AddTorrent undefined result")
				}
			} else {
				logger.Error(err)
			}
		}
	} else {
		if err != nil {
			logger.Error(err)
		} else {
			logger.Error("Unable to init transmission client")
		}
	}
}

func (cr *Observer) getReadyFiles(files []*TorrentFile) []string {
	var filesToSend []string
	for _, file := range files {
		if file != nil {
			fullPath := filepath.Join(cr.FilesPath, file.Name)
			fullPath = filepath.FromSlash(fullPath)
			if stat, err := os.Stat(fullPath); err == nil {
				if stat == nil {
					logger.Warningf("Unable to stat file %s", fullPath)
				} else {
					logger.Debugf("Found ready file %s, size: %d", stat.Name(), stat.Size())
					filesToSend = append(filesToSend, fullPath)
					if err := cr.DB.SetTorrentFileConverting(file.Id); err != nil {
						logger.Error(err)
					}
				}
			}
		}
	}
	return filesToSend
}
