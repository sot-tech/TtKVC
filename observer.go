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
	tr "github.com/hekmon/transmissionrpc"
	"html"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sot-te.ch/HTExtractor"
	tg "sot-te.ch/MTHelper"
	"strconv"
	"strings"
	"syscall"
	"time"
	tmpl "text/template"
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
		Host       string     `json:"host"`
		Port       uint16     `json:"port"`
		Login      string     `json:"login"`
		Password   string     `json:"password"`
		Path       string     `json:"path"`
		Encryption bool       `json:"encryption"`
		Client     *tr.Client `json:"-"`
	} `json:"transmission"`
	FilesPath string   `json:"filespath"`
	DB        Database `json:"db"`
	Telegram  struct {
		ApiId     int32  `json:"apiid"`
		ApiHash   string `json:"apihash"`
		BotToken  string `json:"bottoken"`
		DBPath    string `json:"dbpath"`
		FileStore string `json:"filestorepath"`
		OTPSeed   string `json:"otpseed"`
		Messages  struct {
			tg.TGMessages
			State            string         `json:"state"`
			stateTmpl        *tmpl.Template `json:"-"`
			VideoIgnored     string         `json:"videoignored"`
			videoIgnoredTmpl *tmpl.Template `json:"-"`
			VideoForced      string         `json:"videoforced"`
			videoForcedTmpl  *tmpl.Template `json:"-"`
			KUpload          string         `json:"kupload"`
			kuploadTmpl      *tmpl.Template `json:"-"`
			TUpload          string         `json:"tupload"`
			tuploadTmpl      *tmpl.Template `json:"-"`
		} `json:"msg"`
		Video struct {
			Upload   bool   `json:"upload"`
			TempPath string `json:"temppath"`
		} `json:"video"`
		Client *tg.Telegram `json:"-"`
	} `json:"telegram"`
	Kaltura       Kaltura        `json:"kaltura"`
	ignorePattern *regexp.Regexp `json:"-"`
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
	var index uint
	if isMob, err = cr.DB.GetChatExist(chat); err != nil {
		return "", err
	}
	if isAdmin, err = cr.DB.GetAdminExist(chat); err != nil {
		return "", err
	}
	if index, err = cr.DB.GetCrawlOffset(); err != nil {
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
	return formatMessage(cr.Telegram.Messages.stateTmpl, map[string]interface{}{
		pWatch:        isMob,
		pAdmin:        isAdmin,
		pFilesPending: pendingSB.String(),
		pIndex:        index,
		pVersion:      Version,
	})
}

func (cr *Observer) InitTg() error {
	logger.Debug("Initiating telegram bot")
	telegram := tg.New(cr.Telegram.ApiId, cr.Telegram.ApiHash, cr.Telegram.DBPath, cr.Telegram.FileStore, cr.Telegram.OTPSeed)
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
	if err := telegram.LoginAsBot(cr.Telegram.BotToken, tg.MtLogWarning); err == nil {
		cr.Telegram.Client = telegram
		logger.Debug("Telegram bot init complete")
		return cr.Telegram.Client.AddCommand(tCmdSwitchIgnore, cr.cmdSwitchFileReadyStatus)
	} else {
		return err
	}
}

func (cr *Observer) InitTransmission() error {
	var err error
	logger.Debug("Initiating transmission rpc")
	if !isEmpty(cr.Transmission.Host) && cr.Transmission.Port > 0 {
		var t *tr.Client
		if t, err = tr.New(cr.Transmission.Host, cr.Transmission.Login, cr.Transmission.Password, &tr.AdvancedConfig{
			HTTPS: cr.Transmission.Encryption,
			Port:  cr.Transmission.Port,
		}); err == nil {
			cr.Transmission.Client = t
		}
	} else {
		err = errors.New("invalid transmission connection data")
	}
	logger.Debug("Transmission rpc init complete, err", err)
	return err
}

func (cr *Observer) InitMetaExtractor() error {
	var err error
	if cr.Crawler.MetaExtractor == nil {
		logger.Debug("Initiating meta extractor")
		if len(cr.Crawler.MetaActions) == 0 {
			err = errors.New("extract actions not set")
		} else {
			ex := HTExtractor.New()
			if err = ex.Compile(cr.Crawler.MetaActions); err == nil {
				cr.Crawler.MetaExtractor = ex
			}
		}
		logger.Debug("Meta extractor init complete, err ", err)
	}
	return err
}

func (cr *Observer) InitMessages() error {
	var err error
	if cr.Telegram.Messages.stateTmpl, err = tmpl.New("state").Parse(cr.Telegram.Messages.State); err != nil {
		return err
	}
	if cr.Telegram.Messages.kuploadTmpl, err = tmpl.New("kupload").Parse(cr.Telegram.Messages.KUpload); err != nil {
		return err
	}
	if cr.Telegram.Messages.tuploadTmpl, err = tmpl.New("tupload").Parse(cr.Telegram.Messages.TUpload); err != nil {
		return err
	}
	if cr.Telegram.Messages.videoForcedTmpl, err = tmpl.New("videoForced").Parse(cr.Telegram.Messages.VideoForced); err != nil {
		return err
	}
	if cr.Telegram.Messages.videoIgnoredTmpl, err = tmpl.New("videoIgnored").Parse(cr.Telegram.Messages.VideoIgnored); err != nil {
		return err
	}

	return nil
}

func (cr *Observer) Init() error {
	var err error
	if cr.ignorePattern, err = regexp.Compile(cr.Crawler.IgnoreRegexp); err != nil {
		return err
	}
	if err = cr.DB.Connect(); err != nil {
		return err
	}
	if err = cr.InitTg(); err != nil {
		return err
	}
	if err = cr.InitMessages(); err != nil{
		logger.Error(err)
		err = nil
	}
	if err = cr.InitMetaExtractor(); err != nil {
		logger.Error(err)
		err = nil
	}
	if err = cr.InitTransmission(); err != nil {
		logger.Error(err)
		err = nil
	}
	return nil
}

func (cr *Observer) Engage() {
	var err error
	defer cr.DB.Close()
	var nextOffset uint
	if nextOffset, err = cr.DB.GetCrawlOffset(); err == nil {
		go cr.Telegram.Client.HandleUpdates()
		for {
			newNextOffset := nextOffset
			torrents := make([]*Torrent, 0, cr.Crawler.Threshold)
			for offsetToCheck := nextOffset; offsetToCheck < nextOffset+cr.Crawler.Threshold; offsetToCheck++ {
				var torrent *Torrent
				torrent = cr.checkTorrent(offsetToCheck)
				if torrent != nil {
					newNextOffset = offsetToCheck + 1
					torrents = append(torrents, torrent)
				}
			}
			if newNextOffset > nextOffset {
				nextOffset = newNextOffset
				if err = cr.DB.UpdateCrawlOffset(nextOffset); err != nil {
					logger.Error(err)
				}
			}
			if len(torrents) > 0 {
				go cr.uploadTorrents(torrents)
			}
			cr.checkVideo()
			sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
			logger.Debugf("Sleeping %d sec", sleepTime)
			time.Sleep(sleepTime * time.Second)
		}
	} else {
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
					meta[k] = strings.TrimSpace(html.UnescapeString(string(v)))
				}
			}
		}
	}
	return meta, err
}

func (cr *Observer) checkTorrent(offset uint) *Torrent {
	var err error
	var torrent *Torrent
	logger.Debug("Checking offset ", offset)
	fullContext := fmt.Sprintf(cr.Crawler.ContextURL, offset)
	if torrent, err = GetTorrent(cr.Crawler.BaseURL + fullContext); err == nil {
		if torrent != nil {
			logger.Info("New file", torrent.Info.Name)
			size := torrent.FullSize()
			logger.Info("New torrent size", size)
			if size > 0 {
				if !cr.ignorePattern.MatchString(torrent.Info.Name) {
					files := torrent.Files()
					logger.Debug("Adding torrent", torrent.Info.Name)
					logger.Debug("Files: ", files)
					var id int64
					if id, err = cr.DB.AddTorrent(torrent.Info.Name, offset, files); err == nil {
						var meta map[string]string
						if meta, err = cr.DB.GetTorrentMeta(id); err == nil {
							if len(meta) == 0 {
								if meta, err = cr.getTorrentMeta(fullContext); err == nil && len(meta) > 0 {
									logger.Debug("Writing meta: ", meta)
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
			} else {
				logger.Error("Zero torrent size, offset", offset)
			}
		} else {
			logger.Debugf("%s not a torrent", fullContext)
		}
	}
	return torrent
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
								logger.Debug("Torrent marked as toDelete", *(existingTorrent.Name))
								torrentsToRm = append(torrentsToRm, *existingTorrent.ID)
							}
						}
					}
				}
				if len(torrentsToRm) > 0 {
					if err := cr.Transmission.Client.TorrentRemove(&tr.TorrentRemovePayload{
						IDs:             torrentsToRm,
						DeleteLocalData: false,
					}); err == nil {
						logger.Debug("Torrents deleted", torrentsToRm)
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
			if addedTorrent, err := cr.Transmission.Client.TorrentAdd(&tr.TorrentAddPayload{
				DownloadDir: &cr.Transmission.Path,
				MetaInfo:    &b64,
				Paused:      falsePtr,
			}); err == nil {
				if addedTorrent != nil {
					logger.Debug("Added torrent", *(addedTorrent.Name))
				} else {
					logger.Warning("AddTorrent undefined result", newTorrent.Info.Name)
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
						stat, err = os.Stat(fullPath)
						switch osErr := err.(type) {
						case *os.PathError:
							if osErr.Err == syscall.EINTR {
								stat, err = os.Stat(fullPath)
							} else if osErr.Err == syscall.ENOENT {
								continue
							}
						}
						if err == nil {
							if stat == nil {
								logger.Warning("Unable to stat file", fullPath)
							} else {
								fName := stat.Name()
								logger.Debugf("Found ready file %s, size: %d", fName, stat.Size())
								var entryId string
								if entryId, err = cr.Kaltura.CreateMediaEntry(fullPath); err == nil && !isEmpty(entryId) {
									logger.Debug("Uploading file", fName)
									if err = cr.Kaltura.UploadMediaContent(fullPath, entryId); err == nil {
										logger.Debug("Updating file entry id", entryId)
										if err = cr.DB.SetTorrentFileEntryId(file.Id, entryId); err == nil {
											var admins []int64
											if admins, err = cr.DB.GetAdmins(); err == nil {
												var msg string
												if msg, err = formatMessage(cr.Telegram.Messages.kuploadTmpl,
													map[string]interface{}{
														pName:  filepath.Base(file.Name),
														pId:    entryId,
														pIndex: file.Id,
													}); err != nil{
													msg = err.Error()
												}
												cr.Telegram.Client.SendMsg(msg, admins, true)
												if cr.Telegram.Video.Upload {
													file.Status = FileReadyStatus
												} else {
													file.Status = FileConvertingStatus
												}
												err = cr.switchFileReadyStatus(file, admins)
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
									if err = cr.DB.SetTorrentFileStatus(file.Id, FileReadyStatus); err == nil {
										var flavors KFlavorAssetSearchResult
										if flavors, err = cr.Kaltura.GetMediaEntryFlavorAssets(file.EntryId); err == nil {
											if len(flavors.Objects) == 0 {
												err = errors.New("flavors for entry " + file.EntryId + " not found")
											} else {
												cr.sendTelegramVideo(file, entry, flavors.Objects[0])
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

func (cr *Observer) cmdSwitchFileReadyStatus(chat int64, _, args string) error {
	var err error
	var id int64
	var isAdmin bool
	if isAdmin, err = cr.DB.GetAdminExist(chat); err == nil {
		if isAdmin {
			if id, err = strconv.ParseInt(args, 10, 64); err == nil {
				var file TorrentFile
				if file, err = cr.DB.GetTorrentFile(id); err == nil {
					if isEmpty(file.Name) {
						err = errors.New("no such entry")
					} else {
						err = cr.switchFileReadyStatus(file, []int64{chat})
					}
				}
			}
		} else {
			logger.Infof("SwitchFileReadyStatus unauthorized %d", chat)
			cr.Telegram.Client.SendMsg(cr.Telegram.Messages.Unauthorized, []int64{chat}, false)
		}
	}
	return err
}

func (cr *Observer) switchFileReadyStatus(file TorrentFile, chats []int64) error {
	var err error
	var newFileStatus uint8
	var ignoreMsg *tmpl.Template
	if file.Status == FileReadyStatus {
		ignoreMsg = cr.Telegram.Messages.videoForcedTmpl
		newFileStatus = FileConvertingStatus
	} else {
		ignoreMsg = cr.Telegram.Messages.videoIgnoredTmpl
		newFileStatus = FileReadyStatus
	}
	if err = cr.DB.SetTorrentFileStatus(file.Id, newFileStatus); err == nil {
		var msg string
		if msg, err = formatMessage(ignoreMsg,
			map[string]interface{}{
				pName:   filepath.Base(file.Name),
				pIgnore: tCmdSwitchIgnore + "_" + strconv.FormatInt(file.Id, 10),
			}); err != nil{
			msg = err.Error()
		}
		cr.Telegram.Client.SendMsg(msg, chats, true)
	}
	return err
}

func (cr *Observer) sendTelegramVideo(file TorrentFile, entry KMediaEntry, flavor KFlavorAsset) {
	var err error
	var meta map[string]string
	if meta, err = cr.DB.GetTorrentMeta(file.Torrent); err == nil {
		//if len(meta) == 0{
		//	logger.Warningf("Meta for file %s not found, reloading from target", file.Name)
		//	var offset uint
		//	var merr error
		//	if offset, merr = cr.DB.GetTorrentOffset(file.Torrent); merr == nil && offset > 0{
		//		fullContext := fmt.Sprintf(cr.Crawler.ContextURL, offset)
		//		if meta, merr = cr.getTorrentMeta(fullContext); merr == nil && len(meta) > 0{
		//			merr = cr.DB.AddTorrentMeta(file.Torrent, meta)
		//		}
		//	}
		//	if merr != nil{
		//		logger.Error(merr)
		//	}
		//}
		var chats []int64
		if chats, err = cr.DB.GetChats(); err == nil {
			var index int64
			var msg string
			if index, err = cr.DB.GetTorrentFileIndex(file.Torrent, file.Id); err != nil {
				logger.Error(err)
			}
			if msg, err = formatMessage(cr.Telegram.Messages.tuploadTmpl, map[string]interface{}{
				pMeta: meta,
				pVideoUrl: entry.DownloadURL,
				pIndex: index,
			}); err != nil{
				msg = err.Error()
			}
			var tmpVideoFileName string
			if tmpVideoFileName, err = downloadToDirectory(cr.Telegram.Video.TempPath, entry.DownloadURL, flavor.FileExt);
				err == nil {
				defer os.Remove(tmpVideoFileName)
				var thumb *tg.MediaParams
				if tmpThumbFileName, err := downloadToDirectory(cr.Telegram.Video.TempPath,
					FormatThumbnailURL(entry.ThumbnailUrl, flavor.Width, flavor.Height), "jpg");
					err == nil {
					defer os.Remove(tmpThumbFileName)
					thumb = &tg.MediaParams{
						Path:      tmpThumbFileName,
						Width:     int32(flavor.Width),
						Height:    int32(flavor.Width),
						Streaming: false,
					}
				} else {
					logger.Error(err)
				}
				video := tg.MediaParams{
					Path:      tmpVideoFileName,
					Width:     int32(flavor.Width),
					Height:    int32(flavor.Height),
					Streaming: true,
					Thumbnail: thumb,
				}
				cr.Telegram.Client.SendVideo(video, msg, chats, true)
			}
		}
	}
	if err != nil {
		logger.Error(err)
	}
}
