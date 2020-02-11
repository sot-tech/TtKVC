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
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
)

type Database struct {
	Con *sql.DB
}

const (
	DBDriver = "sqlite3"

	selectChats = "SELECT ID FROM TT_CHAT"
	insertChat  = "INSERT INTO TT_CHAT(ID) VALUES ($1)"
	delChat     = "DELETE FROM TT_CHAT WHERE ID = $1"
	existChat   = "SELECT TRUE FROM TT_CHAT WHERE ID = $1"

	selectTorrent         = "SELECT ID FROM TT_TORRENT WHERE NAME = $1"
	insertOrUpdateTorrent = "INSERT INTO TT_TORRENT(NAME) VALUES ($1) ON CONFLICT(NAME) DO NOTHING"

	selectTorrentFiles         = "SELECT NAME FROM TT_TORRENT_FILE WHERE TORRENT = $1"
	selectTorrentFilesNotReady = "SELECT ID, NAME FROM TT_TORRENT_FILE WHERE READY = 0"
	insertTorrentFile          = "INSERT INTO TT_TORRENT_FILE(TORRENT, NAME) VALUES ($1, $2) ON CONFLICT (TORRENT,NAME) DO NOTHING"
	setTorrentFileReady        = "UPDATE TT_TORRENT_FILE SET READY = 1 WHERE ID = $1"

	selectConfig         = "SELECT VALUE FROM TT_CONFIG WHERE NAME = $1"
	insertOrUpdateConfig = "INSERT INTO TT_CONFIG(NAME, VALUE) VALUES ($1, $2) ON CONFLICT(NAME) DO UPDATE SET VALUE = EXCLUDED.VALUE"

	confCrawlOffset = "CRAWL_OFFSET"
	confTgOffset    = "TG_OFFSET"
)

func (db *Database) checkConnection() error {
	var err error
	if db.Con == nil {
		err = errors.New("connection not initialized")
	} else {
		err = db.Con.Ping()
	}
	return err
}

func (db *Database) getNotEmpty(query string, args ...interface{}) (bool, error) {
	val := false
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Con.Query(query, args...)
		if err == nil && rows != nil {
			defer rows.Close()
			val = rows.Next()
		}
	}
	return val, err
}

func (db *Database) GetChatExist(chat int64) (bool, error) {
	return db.getNotEmpty(existChat, chat)
}

func (db *Database) execNoResult(query string, args ...interface{}) error {
	var err error
	err = db.checkConnection()
	if err == nil {
		_, err = db.Con.Exec(query, args...)
	}
	return err
}

func (db *Database) GetChats() ([]int64, error) {
	var err error
	var chats []int64
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Con.Query(selectChats)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var chat int64
				err = rows.Scan(&chat)
				if err == nil {
					chats = append(chats, chat)
				} else {
					break
				}
			}
		}
	}
	return chats, err
}

func (db *Database) AddChat(chat int64) error {
	var exist bool
	var err error
	if exist, err = db.GetChatExist(chat); err == nil && !exist {
		err = db.execNoResult(insertChat, chat)
	}
	return err
}

func (db *Database) DelChat(chat int64) error {
	return db.execNoResult(delChat, chat)
}

func (db *Database) GetTorrent(torrent string) (int64, []string, error) {
	var torrentId int64
	var torrentFiles []string
	var err error
	torrentId = -1
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Con.Query(selectTorrent, torrent)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&torrentId)
				if err == nil {
					var rows *sql.Rows
					rows, err = db.Con.Query(selectTorrentFiles, torrentId)
					if err == nil && rows != nil {
						defer rows.Close()
						for rows.Next() {
							var element string
							if err := rows.Scan(&element); err == nil {
								torrentFiles = append(torrentFiles, element)
							} else {
								break
							}
						}
					}
				}
			}
		}
	}
	return torrentId, torrentFiles, err
}

func (db *Database) AddTorrent(name string, files []string) error {
	var err error
	if err = db.execNoResult(insertOrUpdateTorrent, name); err == nil {
		var id int64
		if id, _, err = db.GetTorrent(name); err == nil {
			for _, file := range files {
				err = db.execNoResult(insertTorrentFile, id, file)
			}
		}
	}
	return err
}

type TorrentFile struct {
	Id   uint64
	Name string
}

func (db *Database) GetTorrentFilesNotReady() ([]TorrentFile, error) {
	var err error
	var files []TorrentFile
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Con.Query(selectTorrentFilesNotReady)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var file TorrentFile
				if err = rows.Scan(&file); err != nil{
					break
				}
			}
		}
	}
	return files, err
}

func (db *Database) SetTorrentFileReady(id uint64) error {
	return db.execNoResult(setTorrentFileReady, id)
}

func (db *Database) getConfigValue(name string) (string, error) {
	var val string
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Con.Query(selectConfig, name)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&val)
			}
		}
	}
	return val, err
}

func (db *Database) updateConfigValue(name, val string) error {
	return db.execNoResult(insertOrUpdateConfig, name, val)
}

func (db *Database) GetCrawlOffset() (uint, error) {
	var res uint64
	var val string
	var err error
	if val, err = db.getConfigValue(confCrawlOffset); err == nil {
		res, err = strconv.ParseUint(val, 10, 64)
	}
	return uint(res), err
}

func (db *Database) UpdateCrawlOffset(offset uint) error {
	return db.updateConfigValue(confCrawlOffset, strconv.FormatUint(uint64(offset), 10))
}

func (db *Database) GetTgOffset() (int, error) {
	var res int64
	var val string
	var err error
	if val, err = db.getConfigValue(confCrawlOffset); err == nil {
		res, err = strconv.ParseInt(val, 10, 64)
	}
	return int(res), err
}

func (db *Database) UpdateTgOffset(offset int) error {
	return db.updateConfigValue(confTgOffset, strconv.FormatUint(uint64(offset), 10))
}

func (db *Database) Connect(path string) error {
	var err error
	db.Con, err = sql.Open(DBDriver, path)
	if err == nil {
		err = db.checkConnection()
	}
	return err
}

func (db *Database) Close() {
	if db.Con != nil {
		_ = db.Con.Close()
	}
}
