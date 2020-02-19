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
	"errors"
	"fmt"
	trans "github.com/hekmon/transmissionrpc"
	"github.com/sot-tech/telegram-bot-api"
	"html"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sot-te.ch/HTExtractor"
	tg "sot-te.ch/TGHelper"
	"strconv"
	"strings"
	"time"
)

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		BaseURL       string                      `json:"baseurl"`
		ContextURL    string                      `json:"contexturl"`
		Delay         uint                        `json:"delay"`
		Threshold     uint                        `json:"threshold"`
		IgnoreRegexp  string                      `json:"ignoreregexp"`
		MetaActions   []HTExtractor.ExtractAction `json:"metaactions"`
		MetaExtractor *HTExtractor.Extractor      `json:"-"`
	} `json:"crawler"`
	Transmission struct {
		Host       string `json:"host"`
		Port       uint16 `json:"port"`
		Login      string `json:"login"`
		Password   string `json:"password"`
		Path       string `json:"path"`
		Encryption bool   `json:"encryption"`
		Client     *trans.Client
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
		Video struct {
			Upload         bool     `json:"upload"`
			SizeThreshold  uint64   `json:"sizethreshold"`
			CustomUploader []string `json:"customuploader"`
		} `json:"video"`
		Client *tg.Telegram `json:"-"`
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
	var pending []TorrentFile
	if isMob, err = cr.DB.GetChatExist(chat); err != nil {
		return "", err
	}
	if isAdmin, err = cr.DB.GetAdminExist(chat); err != nil {
		return "", err
	}
	pendingSB := strings.Builder{}
	if strings.Index(cr.Telegram.Messages.State, pFilesPending) >= 0 {
		if pending, err = cr.DB.GetTorrentFilesNotReady(); err != nil {
			return "", err
		}
		if pending != nil {
			for _, val := range pending {
				pendingSB.WriteString(val.String())
				pendingSB.WriteRune('\n')
			}
		}
	}
	return formatMessage(cr.Telegram.Messages.State, map[string]interface{}{
		pWatch:        isMob,
		pAdmin:        isAdmin,
		pFilesPending: pendingSB.String(),
	}), err
}

func (cr *Observer) initTg() {
	logger.Debug("Initiating telegram bot")
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
	cr.Telegram.Client = telegram
	logger.Debug("Telegram bot init complete")
}

func (cr *Observer) initTransmission() error {
	var err error
	logger.Debug("Initiating transmission rpc")
	if !isEmpty(cr.Transmission.Host) || cr.Transmission.Port > 0 {
		var t *trans.Client
		if t, err = trans.New(cr.Transmission.Host, cr.Transmission.Login, cr.Transmission.Password, &trans.AdvancedConfig{
			HTTPS: cr.Transmission.Encryption,
			Port:  cr.Transmission.Port,
		}); err == nil {
			cr.Transmission.Client = t
		}
	} else {
		err = errors.New("invalid transmission connection data")
	}
	logger.Debugf("Transmission rpc init complete, err %v", err)
	return err
}

func (cr *Observer) initMetaExtractor() error {
	var err error
	if cr.Crawler.MetaExtractor == nil {
		logger.Debug("Initiating meta extractor")
		if cr.Crawler.MetaActions == nil {
			err = errors.New("extract actions not set")
		} else {
			ex := HTExtractor.New()
			if err = ex.Compile(cr.Crawler.MetaActions); err == nil {
				cr.Crawler.MetaExtractor = ex
			}
		}
		logger.Debugf("Meta extractor init complete, err %v", err)
	}
	return err
}

func (cr *Observer) Engage() {
	var err error
	ignorePattern := regexp.MustCompile(cr.Crawler.IgnoreRegexp)
	if err = cr.DB.Connect(); err == nil {
		defer cr.DB.Close()
		cr.initTg()
		if err = cr.Telegram.Client.Connect(cr.Telegram.Token, -1); err == nil {
			var baseOffset uint
			go cr.Telegram.Client.HandleUpdates()
			if baseOffset, err = cr.DB.GetCrawlOffset(); err == nil {
				if err := cr.initMetaExtractor(); err != nil {
					logger.Error(err)
				}
				if err := cr.initTransmission(); err != nil {
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
					go cr.checkVideo()
					sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
					logger.Debugf("Sleeping %d sec", sleepTime)
					time.Sleep(sleepTime * time.Second)
				}
			}
		}
	}
	if err != nil {
		logger.Fatal(err)
	}
}

func (cr *Observer) getTorrentMeta(context string) (map[string]string, error) {
	var err error
	var meta map[string]string
	if cr.Crawler.MetaExtractor != nil {
		var rawMeta map[string][]byte
		if rawMeta, err = cr.Crawler.MetaExtractor.ExtractData(cr.Crawler.BaseURL, context); err == nil && len(rawMeta) > 0 {
			meta = make(map[string]string, len(rawMeta))
			for k, v := range rawMeta {
				if !isEmpty(k) {
					meta[k] = html.UnescapeString(string(v))
				}
			}
		}
	}
	return meta, err
}

func (cr *Observer) checkTorrent(currentOffset uint, ignorePattern *regexp.Regexp) (uint, *Torrent) {
	var err error
	var torrent *Torrent
	logger.Debugf("Checking offset %d", currentOffset)
	fullContext := fmt.Sprintf(cr.Crawler.ContextURL, currentOffset)
	if torrent, err = GetTorrent(cr.Crawler.BaseURL + fullContext); err == nil {
		if torrent != nil {
			logger.Infof("New file %s", torrent.Info.Name)
			size := torrent.FullSize()
			logger.Infof("New torrent size %d", size)
			if size > 0 {
				if !ignorePattern.MatchString(torrent.Info.Name) {
					files := torrent.Files()
					logger.Debugf("Adding torrent %s", torrent.Info.Name)
					logger.Debugf("Files: %v", files)
					var id int64
					if id, err = cr.DB.AddTorrent(torrent.Info.Name, files); err == nil {
						var meta map[string]string
						if meta, err = cr.DB.GetTorrentMeta(id); err == nil {
							if len(meta) == 0 {
								if meta, err = cr.getTorrentMeta(fullContext); err == nil && len(meta) > 0 {
									logger.Debugf("Writing meta: %v", meta)
									err = cr.DB.AddTorrentMeta(id, meta)
								}
							}
						}
					}
					if err != nil {
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
			logger.Debugf("%s not a torrent", fullContext)
		}
	}
	return currentOffset, torrent
}

func (cr *Observer) uploadTorrents(newTorrents []*Torrent) {
	if cr.Transmission.Client != nil {
		if existingTorrents, err := cr.Transmission.Client.TorrentGet([]string{"id", "name"}, nil); err == nil {
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
					if err := cr.Transmission.Client.TorrentRemove(&trans.TorrentRemovePayload{
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
			if addedTorrent, err := cr.Transmission.Client.TorrentAdd(&trans.TorrentAddPayload{
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
		logger.Warning("Transmission client not inited")
	}
}

func (cr *Observer) checkVideo() {
	var err error
	if err = cr.Kaltura.CreateSession(); err == nil {
		defer cr.Kaltura.EndSession()
		var files []TorrentFile
		if files, err = cr.DB.GetTorrentFilesNotReady(); err == nil && files != nil {
			for _, file := range files {
				if !isEmpty(file.Name) {
					if file.Status == FilePendingStatus {
						var err error
						fullPath := filepath.Join(cr.FilesPath, file.Name)
						fullPath = filepath.FromSlash(fullPath)
						var stat os.FileInfo
						if stat, err = os.Stat(fullPath); err == nil {
							if stat == nil {
								logger.Warningf("Unable to stat file %s", fullPath)
							} else {
								logger.Debugf("Found ready file %s, size: %d", stat.Name(), stat.Size())
								var entryId string
								if entryId, err = cr.Kaltura.CreateMediaEntry(fullPath); err == nil && !isEmpty(entryId) {
									if err = cr.Kaltura.UploadMediaContent(fullPath, entryId); err == nil {
										if err = cr.DB.SetTorrentFileConverting(file.Id); err == nil {
											if err = cr.DB.SetTorrentFileEntryId(file.Id, entryId); err == nil {
												var admins []int64
												if admins, err = cr.DB.GetAdmins(); err == nil {
													cr.Telegram.Client.SendMsg(formatMessage(cr.Telegram.Messages.KUpload,
														map[string]interface{}{
															pName: filepath.Base(file.Name),
															pId:   entryId,
														}), admins, true)
												}
											}
										}
									}
								}
							}
						}
						if err != nil {
							logger.Error(err)
						}
					} else if file.Status == FileConvertingStatus {
						var err error
						if isEmpty(file.EntryId) {
							err = errors.New("entry id not set for file " + file.String())
						} else {
							var entry KMediaEntry
							if entry, err = cr.Kaltura.GetMediaEntry(file.EntryId); err == nil {
								if entry.Status == KEntryStatusReady {
									if err = cr.DB.SetTorrentFileReady(file.Id); err == nil {
										var flavors KFlavorAssetSearchResult
										if flavors, err = cr.Kaltura.GetMediaEntryFlavorAssets(file.EntryId); err == nil {
											if len(flavors.Objects) > 0 {
												cr.sendTelegramVideo(file, entry, flavors.Objects[0])
											} else {
												err = errors.New("flavors for entry " + file.EntryId + " not found")
											}
										}
									}
								}
							}
						}
						if err != nil {
							logger.Error(err)
						}
					}
				}
			}
		}
	}
	if err != nil {
		logger.Error(err)
	}
}

func (cr *Observer) sendTelegramVideo(file TorrentFile, entry KMediaEntry, flavor KFlavorAsset) {
	var err error
	var meta map[string]string
	if meta, err = cr.DB.GetTorrentMeta(file.Torrent); err == nil {
		var chats []int64
		if chats, err = cr.DB.GetChats(); err == nil {
			var index int64
			if index, err = cr.DB.GetTorrentFileIndex(file.Torrent, file.Id); err != nil {
				logger.Error(err)
			}
			replacements := make(map[string]interface{}, len(meta)+2)
			for k, v := range meta {
				replacements["${" + k + "}"] = v
			}
			replacements[pVideoUrl] = entry.DownloadURL
			replacements[pIndex] = strconv.FormatInt(index, 10)
			msg := formatMessage(cr.Telegram.Messages.TUpload, replacements)
			if cr.Telegram.Video.Upload {
				if flavor.Size*1024 < cr.Telegram.Video.SizeThreshold {
					r, w := io.Pipe()
					defer r.Close()
					go func() {
						defer w.Close()
						if err == nil {
							if resp, httpErr := http.Get(entry.DownloadURL); checkResponse(resp, httpErr) {
								defer resp.Body.Close()
								_, err = io.Copy(w, resp.Body)
							} else {
								err = responseError(resp, httpErr)
							}
						}
					}()
					video := tgbotapi.BaseFile{
						File:     r,
						MimeType: "video/" + flavor.FileExt,
						FileSize: int(flavor.Size * 1024),
					}
					cr.Telegram.Client.SendVideo(msg, video, chats, true)
				} else if len(cr.Telegram.Video.CustomUploader) > 1 {
					logger.Debugf("Using custom uploader %v", cr.Telegram.Video.CustomUploader)
					if len(chats) > 0 {
						firstChat := chats[0]
						anotherChats := chats[1:]
						cmd := cr.Telegram.Video.CustomUploader[0]
						args := make([]string, len(cr.Telegram.Video.CustomUploader)-1)
						copy(args, cr.Telegram.Video.CustomUploader[1:])
						for i := 0; i < len(args); i++ {
							switch args[i] {
							case pVideoUrl:
								args[i] = strings.ReplaceAll(args[i], pVideoUrl, entry.DownloadURL)
							case pId:
								args[i] = strings.ReplaceAll(args[i], pId, strconv.FormatInt(firstChat, 10))
							case pMsg:
								args[i] = strings.ReplaceAll(args[i], pMsg, msg)
							}
						}
						if proc := exec.Command(cmd, args...); proc != nil {
							var stdout []byte
							if stdout, err = proc.Output(); err == nil {
								if videoId := string(stdout); !isEmpty(videoId) {
									video := tgbotapi.BaseFile{
										FileID:      videoId,
										UseExisting: true,
									}
									cr.Telegram.Client.SendVideo(msg, video, anotherChats, true)
								} else {
									err = errors.New("no video_id got from custom uploader")
								}
							}
						} else {
							err = errors.New("unable to create process for " + cmd)
						}
					} else {
						logger.Warning("Custom uploader not set, video file is too big")
					}
				}
			} else {
				cr.Telegram.Client.SendMsg(msg, chats, true)
			}
		}
	}
	if err != nil {
		logger.Error(err)
	}
}
