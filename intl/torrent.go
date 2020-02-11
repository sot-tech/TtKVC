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

package intl

import (
	"github.com/zeebo/bencode"
	"net/http"
	"path/filepath"
)

type Torrent struct {
	AnnounceList [][]string `bencode:"announce-list"`
	Announce     string     `bencode:"announce"`
	Comment      string     `bencode:"comment"`
	CreatedBy    string     `bencode:"created by"`
	CreationDate int64      `bencode:"creation date"`
	Publisher    string     `bencode:"publisher"`
	PublisherUrl string     `bencode:"publisher-url"`
	Encoding     string     `bencode:"encoding"`
	Info         struct {
		Length uint64 `bencode:"length"`
		Files  []struct {
			Length uint64   `bencode:"length"`
			Path   []string `bencode:"path"`
		} `bencode:"files"`
		Name        string `bencode:"name"`
		PieceLength uint64 `bencode:"piece length"`
		Pieces      []byte `bencode:"pieces"`
	} `bencode:"info"`
}

func GetTorrent(url string) (*Torrent, error) {
	var res *Torrent
	var err error
	if resp, httpErr := http.Get(url); checkResponse(resp, httpErr) {
		var torrent Torrent
		err := bencode.NewDecoder(resp.Body).Decode(&torrent)
		if err == nil {
			res = &torrent
		}
	} else {
		err = responseError(resp, httpErr)
	}
	return res, err
}

func (t *Torrent) FullSize() uint64 {
	var fullLen uint64
	if t.Info.Length > 0 {
		fullLen = t.Info.Length
	} else {
		if t.Info.Files != nil {
			for _, file := range t.Info.Files {
				fullLen += file.Length
			}
		}
	}
	return fullLen
}

func (t *Torrent) Files() []string{
	var files []string
	if t.Info.Files != nil{
		for _, file := range t.Info.Files{
			if file.Path != nil{
				allParts := []string{t.Info.Name}
				allParts = append(allParts, file.Path...)
				files = append(files, "/" + filepath.Join(allParts...))
			}
		}
	} else{
		files = append(files, "/" + t.Info.Name)
	}
	return files
}
