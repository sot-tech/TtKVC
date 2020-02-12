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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	jsonMime            = "application/json"
	kAPISessionStart    = "api_v3/service/session/action/start?format=1"
	kAPISessionEnd      = "api_v3/service/session/action/end?format=1&ks=%s"
	kAPIMediaAdd        = "api_v3/service/media/action/add?format=1&ks=%s"
	kAPIMediaAddContent = "api_v3/service/media/action/addContent?format=1&ks=%s"
	kSessionTTL         = 600
	kUserSessionType    = 0
	kVideoMediaType     = 1
	kFileSourceType     = "1"
	kMediaEntryType     = "KalturaMediaEntry"
	kObjectTypeFiled    = "resource:objectType"
	kFileDataField      = "resource:fileData"
	kUploadFileResource = "KalturaUploadedFileResource"
	kEntryId            = "entryId"
)

type Kaltura struct {
	URL       string    `json:"url"`
	PartnerId uint      `json:"partnerid"`
	UserId    string    `json:"userid"`
	Secret    string    `json:"secret"`
	Telegram  *Telegram `json:"-"`
}

type kSession struct {
	Secret     string `json:"secret"`
	UserID     string `json:"userId"`
	Type       uint   `json:"type"`
	PartnerID  uint   `json:"partnerId"`
	Expiry     int64  `json:"expiry"`
	Privileges string `json:"privileges"`
}

type kEntry struct {
	Id         string `json:"id,omitempty"`
	Name       string `json:"name"`
	UserId     string `json:"userId"`
	CreatorId  string `json:"creatorId"`
	ObjectType string `json:"objectType"`
	MediaType  uint   `json:"mediaType"`
	SourceType string `json:"sourceType"`
}

type kMediaEntry struct {
	Entry kEntry `json:"entry"`
}

type kError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	ObjectType string `json:"objectType"`
	Args       []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"args"`
}

func (kl *Kaltura) prepareURL(context string) string{
	delimiter := ""
	if strings.LastIndexByte(kl.URL, '/') != len(kl.URL)-1 {
		delimiter = "/"
	}
	return fmt.Sprintf("%s%s%s", kl.URL, delimiter, context)
}

func (kl *Kaltura) postJson(context string, obj interface{}) ([]byte, error) {
	var err error
	var data []byte
	fullUrl := kl.prepareURL(context)
	if data, err = json.Marshal(obj); err == nil {
		if resp, httpErr := http.Post(fullUrl, jsonMime, bytes.NewReader(data)); checkResponse(resp, httpErr){
			data, err = ioutil.ReadAll(resp.Body)
		} else {
			err = responseError(resp, httpErr)
		}
	}
	return data, err
}

func (kl *Kaltura) createSession(ttlMlx int64) (string, error) {
	var err error
	var session string
	obj := kSession{
		Secret:     kl.Secret,
		UserID:     kl.UserId,
		Type:       kUserSessionType,
		PartnerID:  kl.PartnerId,
		Expiry:     time.Now().Unix() + (kSessionTTL * ttlMlx),
		Privileges: "*",
	}
	var data []byte
	if data, err = kl.postJson(kAPISessionStart, obj); err == nil {
		if err = jsonError(data); err == nil {
			session = string(data)
			session = strings.Replace(session, "\"", "", -1)
			err = nil
		}
	}
	return session, err
}

func (kl *Kaltura) endSession(session string) {
	fullUrl := kl.prepareURL(kAPISessionEnd)
	fullUrl = fmt.Sprintf(fullUrl, session)
	if resp, err := http.Get(fullUrl); !checkResponse(resp, err) {
		Logger.Error(responseError(resp, err))
	}
}

func (kl *Kaltura) createMediaEntry(session, name string) (string, error) {
	var err error
	var entryId string
	fullUrl := fmt.Sprintf(kAPIMediaAdd, session)
	obj := kMediaEntry{
		Entry: kEntry{
			Name:       filepath.Base(name),
			UserId:     kl.UserId,
			CreatorId:  kl.UserId,
			ObjectType: kMediaEntryType,
			MediaType:  kVideoMediaType,
			SourceType: kFileSourceType,
		},
	}
	var data []byte
	if data, err = kl.postJson(fullUrl, obj); err == nil {
		entry := kEntry{}
		if err = json.Unmarshal(data, &entry); err == nil {
			entryId = entry.Id
		} else {
			if err = jsonError(data); err == nil {
				err = errors.New("Unknown response: " + string(data))
			}
		}
	}
	if entryId == "" {
		err = errors.New("unable to get entry id")
	}
	return entryId, err
}

func jsonError(data []byte) error {
	var err error
	outErr := kError{}
	if err = json.Unmarshal(data, &outErr); err == nil {
		if outErr.Code != "" {
			err = errors.New(outErr.ObjectType + ":" + outErr.Message)
		}
	} else {
		err = nil
	}
	return err
}

func (kl *Kaltura) uploadMediaContent(session, name, entryId string) error {
	r, w := io.Pipe()
	m := multipart.NewWriter(w)
	var err error
	go func() {
		defer w.Close()
		defer m.Close()
		if err == nil {
			if err := m.WriteField(kEntryId, entryId); err != nil {
				Logger.Error(err)
				return
			}
			if err := m.WriteField(kObjectTypeFiled, kUploadFileResource); err != nil {
				Logger.Error(err)
				return
			}
			part, err := m.CreateFormFile(kFileDataField, filepath.Base(name))
			if err != nil {
				Logger.Error(err)
				return
			}
			file, err := os.Open(name)
			if err != nil {
				Logger.Error(err)
				return
			}
			if file != nil {
				defer file.Close()
				if _, err = io.Copy(part, file); err != nil {
					Logger.Error(err)
					return
				}
			} else {
				Logger.Error("File object is nil")
			}
		}
	}()
	fullUrl := kl.prepareURL(kAPIMediaAddContent)
	fullUrl = fmt.Sprintf(fullUrl, session)
	var resp *http.Response
	if resp, err = http.Post(fullUrl, m.FormDataContentType(), r); checkResponse(resp, err) {
		var data []byte
		if data, err = ioutil.ReadAll(resp.Body); err == nil {
			if err = jsonError(data); err == nil {
				entry := kEntry{}
				if err = json.Unmarshal(data, &entry); err == nil {
					Logger.Debug(entry)
				}
			}
		}
	} else {
		err = responseError(resp, err)
	}
	return err
}

func (kl *Kaltura) ProcessFiles(files []string) {
	if kl.URL == "" || kl.Secret == "" || kl.UserId == "" {
		Logger.Error("Required kaltura parameters not set")
	} else {
		if session, err := kl.createSession(int64(len(files))); err == nil {
			defer kl.endSession(session)
			for _, f := range files {
				if entryId, err := kl.createMediaEntry(session, f); err == nil {
					msg := Messages.Announce
					msg = strings.Replace(msg, msgId, entryId, -1)
					msg = strings.Replace(msg, msgName, filepath.Base(f), -1)
					go kl.Telegram.SendMsgToAll(msg)
					if err := kl.uploadMediaContent(session, f, entryId); err != nil {
						Logger.Error(err)
					}
				} else {
					Logger.Error(err)
				}
			}
		} else {
			Logger.Error(err)
		}
	}
}
