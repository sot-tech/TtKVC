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
	kAPIMediaGet        = "api_v3/service/media/action/get?format=1&ks=%s"
	kAPIMediaAdd        = "api_v3/service/media/action/add?format=1&ks=%s"
	kAPIMediaAddContent = "api_v3/service/media/action/addContent?format=1&ks=%s"
	kAPIFlavorsList     = "api_v3/service/flavorAsset/action/List?format=1&ks=%s"
	kSessionTTL         = 1800
	kUserSessionType    = 0
	kVideoMediaType     = 1
	kFileSourceType     = "1"
	KEntryStatusReady   = 2
	kEntryIdField       = "entryId"
)

type Kaltura struct {
	URL       string `json:"url"`
	PartnerId uint   `json:"partnerid"`
	UserId    string `json:"userid"`
	Secret    string `json:"secret"`
	session   string `json:"-"`
}

type KSession struct {
	Secret     string `json:"secret"`
	UserID     string `json:"userId"`
	Type       uint   `json:"type"`
	PartnerID  uint   `json:"partnerId"`
	Expiry     int64  `json:"expiry"`
	Privileges string `json:"privileges"`
}

type KObject struct {
	Id          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	Description string `json:"description,omitempty"`
}

type KFlavorAssetSearchResult struct {
	KObject
	TotalCount uint64         `json:"totalCount"`
	Objects    []KFlavorAsset `json:"objects"`
}

type KFilter struct {
	Filter interface{} `json:"filter"`
}

type KBaseEntry struct {
	KObject
	UserId      string `json:"userId"`
	CreatorId   string `json:"creatorId"`
	Status      int    `json:"status"`
	DownloadURL string `json:"downloadUrl"`
}

type KMediaEntry struct {
	KBaseEntry
	MediaType  uint   `json:"mediaType"`
	SourceType string `json:"sourceType"`
}

type KFlavorAsset struct {
	KObject
	FlavorParamsID  int64   `json:"flavorParamsId"`
	Width           uint    `json:"width"`
	Height          uint    `json:"height"`
	Bitrate         uint    `json:"bitrate"`
	FrameRate       float64 `json:"frameRate"`
	IsOriginal      bool    `json:"isOriginal"`
	IsWeb           bool    `json:"isWeb"`
	ContainerFormat string  `json:"containerFormat"`
	VideoCodecID    string  `json:"videoCodecId"`
	Status          int     `json:"status"`
	Language        string  `json:"language"`
	IsDefault       bool    `json:"isDefault"`
	EntryID         string  `json:"entryId"`
	PartnerID       uint    `json:"partnerId"`
	Version         string  `json:"version"`
	Size            uint64  `json:"size"`
	Tags            string  `json:"tags"`
	FileExt         string  `json:"fileExt"`
	CreatedAt       int64   `json:"createdAt"`
	UpdatedAt       int64   `json:"updatedAt"`
}

type KError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	ObjectType string `json:"objectType"`
	Args       []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"args"`
}

func (kl *Kaltura) prepareURL(context string) string {
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
		if resp, httpErr := http.Post(fullUrl, jsonMime, bytes.NewReader(data)); checkResponse(resp, httpErr) {
			defer resp.Body.Close()
			data, err = ioutil.ReadAll(resp.Body)
		} else {
			err = responseError(resp, httpErr)
		}
	}
	return data, err
}

func (kl *Kaltura) CreateSession() error {
	if kl.session != "" {
		kl.EndSession()
	}
	var err error
	obj := KSession{
		Secret:     kl.Secret,
		UserID:     kl.UserId,
		Type:       kUserSessionType,
		PartnerID:  kl.PartnerId,
		Expiry:     time.Now().Unix() + kSessionTTL,
		Privileges: "*",
	}
	var data []byte
	if data, err = kl.postJson(kAPISessionStart, obj); err == nil {
		if err = jsonError(data); err == nil {
			kl.session = strings.Replace(string(data), "\"", "", -1)
			err = nil
		}
	}
	return err
}

func (kl *Kaltura) EndSession() {
	if kl.session != "" {
		fullUrl := kl.prepareURL(kAPISessionEnd)
		fullUrl = fmt.Sprintf(fullUrl, kl.session)
		if resp, err := http.Get(fullUrl); !checkResponse(resp, err) {
			logger.Error(responseError(resp, err))
		}
		kl.session = ""
	}
}

type kFlavorByEntryFilter struct {
	KObject
	EntryId string `json:"entryIdEqual"`
}

func (kl *Kaltura) kSend(context string, send interface{}, result interface{}) error {
	var err error
	if kl.session == "" {
		return errors.New("empty session")
	}
	fullContext := fmt.Sprintf(context, kl.session)
	var data []byte
	if data, err = kl.postJson(fullContext, send); err == nil {
		if err = jsonError(data); err == nil {
			err = json.Unmarshal(data, result)
		}
	}
	return err
}

func (kl *Kaltura) GetMediaEntryFlavorAssets(id string) (KFlavorAssetSearchResult, error) {
	var err error
	var res KFlavorAssetSearchResult
	obj := KFilter{Filter: kFlavorByEntryFilter{
		KObject: KObject{
			ObjectType: "KalturaAssetFilter",
		},
		EntryId: id,
	}}
	err = kl.kSend(kAPIFlavorsList, obj, &res)
	return res, err
}

func (kl *Kaltura) GetMediaEntry(id string) (KMediaEntry, error) {
	var err error
	var entry KMediaEntry
	obj := map[string]string{kEntryIdField: id}
	err = kl.kSend(kAPIMediaGet, obj, &entry)
	return entry, err
}

func (kl *Kaltura) CreateMediaEntry(name string) (string, error) {
	var err error
	var entry KMediaEntry
	var entryId string
	obj := map[string]interface{}{
		"entry": KMediaEntry{
			KBaseEntry: KBaseEntry{
				KObject: KObject{
					Name:       filepath.Base(name),
					ObjectType: "KalturaMediaEntry",
				},
				UserId:    kl.UserId,
				CreatorId: kl.UserId,
			},
			MediaType:  kVideoMediaType,
			SourceType: kFileSourceType,
		},
	}
	if err = kl.kSend(kAPIMediaAdd, obj, &entry); err == nil {
		if entry.Id == "" {
			err = errors.New("unable to get entry id")
		} else {
			entryId = entry.Id
		}
	}
	return entryId, err
}

func jsonError(data []byte) error {
	var err error
	outErr := KError{}
	if err = json.Unmarshal(data, &outErr); err == nil {
		if strings.Contains(outErr.ObjectType, "Exception") || outErr.Code != "" {
			err = errors.New(outErr.ObjectType + ":" + outErr.Message)
		}
	} else {
		err = nil
	}
	return err
}

func (kl *Kaltura) UploadMediaContent(name, entryId string) error {
	if kl.session == "" {
		return errors.New("empty session")
	}
	r, w := io.Pipe()
	defer r.Close()
	m := multipart.NewWriter(w)
	var err error
	go func() {
		defer w.Close()
		defer m.Close()
		if err == nil {
			if err := m.WriteField(kEntryIdField, entryId); err != nil {
				logger.Error(err)
				return
			}
			if err := m.WriteField("resource:objectType", "KalturaUploadedFileResource"); err != nil {
				logger.Error(err)
				return
			}
			part, err := m.CreateFormFile("resource:fileData", filepath.Base(name))
			if err != nil {
				logger.Error(err)
				return
			}
			file, err := os.Open(name)
			if err != nil {
				logger.Error(err)
				return
			}
			if file != nil {
				defer file.Close()
				if _, err = io.Copy(part, file); err != nil {
					logger.Error(err)
					return
				}
			} else {
				logger.Error("File object is nil")
			}
		}
	}()
	fullUrl := kl.prepareURL(kAPIMediaAddContent)
	fullUrl = fmt.Sprintf(fullUrl, kl.session)
	var resp *http.Response
	if resp, err = http.Post(fullUrl, m.FormDataContentType(), r); checkResponse(resp, err) {
		defer resp.Body.Close()
		var data []byte
		if data, err = ioutil.ReadAll(resp.Body); err == nil {
			if err = jsonError(data); err == nil {
				entry := KMediaEntry{}
				if err = json.Unmarshal(data, &entry); err == nil {
					logger.Debug(entry)
				}
			}
		}
	} else {
		err = responseError(resp, err)
	}
	return err
}
