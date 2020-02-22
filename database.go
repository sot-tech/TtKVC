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
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
)

type Database struct {
	ConnectionString string  `json:"connection"`
	Connection       *sql.DB `json:"-"`
}

const (
	DBDriver = "sqlite3"

	selectChats = "SELECT ID FROM TT_CHAT"
	insertChat  = "INSERT INTO TT_CHAT(ID) VALUES ($1)"
	delChat     = "DELETE FROM TT_CHAT WHERE ID = $1"
	existChat   = "SELECT TRUE FROM TT_CHAT WHERE ID = $1"

	selectAdmins = "SELECT ID FROM TT_ADMIN"
	insertAdmin  = "INSERT INTO TT_ADMIN(ID) VALUES ($1)"
	delAdmin     = "DELETE FROM TT_ADMIN WHERE ID = $1"
	existAdmin   = "SELECT 1 FROM TT_ADMIN WHERE ID = $1"

	selectTorrent         = "SELECT ID FROM TT_TORRENT WHERE NAME = $1"
	insertOrUpdateTorrent = "INSERT INTO TT_TORRENT(NAME) VALUES ($1) ON CONFLICT(NAME) DO NOTHING"

	selectTorrentMeta = "SELECT NAME, VALUE FROM TT_TORRENT_META WHERE TORRENT = $1"
	insertTorrentMeta = "INSERT INTO TT_TORRENT_META(TORRENT, NAME, VALUE) VALUES($1, $2, $3) ON CONFLICT(TORRENT,NAME) DO UPDATE SET VALUE = EXCLUDED.VALUE"

	selectTorrentFile = "SELECT ID, TORRENT, NAME, ENTRY_ID, READY FROM TT_TORRENT_FILE WHERE ID = $1"
	selectTorrentFilesNotReady = "SELECT ID, TORRENT, NAME, ENTRY_ID, READY FROM TT_TORRENT_FILE WHERE READY != $1"
	selectTorrentFileIndex     = "SELECT IND FROM (SELECT ID, NAME, ROW_NUMBER() OVER(ORDER BY NAME) AS IND FROM TT_TORRENT_FILE WHERE TORRENT = $1) WHERE ID = $2"
	insertTorrentFile          = "INSERT INTO TT_TORRENT_FILE(TORRENT, NAME) VALUES ($1, $2) ON CONFLICT (TORRENT,NAME) DO NOTHING"
	setTorrentFileStatus       = "UPDATE TT_TORRENT_FILE SET READY = $1 WHERE ID = $2"
	setTorrentFileEntryId      = "UPDATE TT_TORRENT_FILE SET ENTRY_ID = $1 WHERE ID = $2"

	selectConfig         = "SELECT VALUE FROM TT_CONFIG WHERE NAME = $1"
	insertOrUpdateConfig = "INSERT INTO TT_CONFIG(NAME, VALUE) VALUES ($1, $2) ON CONFLICT(NAME) DO UPDATE SET VALUE = EXCLUDED.VALUE"

	confCrawlOffset = "CRAWL_OFFSET"
	confTgOffset    = "TG_OFFSET"

	FilePendingStatus    = 0
	FileConvertingStatus = 1
	FileReadyStatus      = 2
)

func (db *Database) checkConnection() error {
	var err error
	if db.Connection == nil {
		err = errors.New("connection not initialized")
	} else {
		err = db.Connection.Ping()
	}
	return err
}

func (db *Database) getNotEmpty(query string, args ...interface{}) (bool, error) {
	val := false
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(query, args...)
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
		_, err = db.Connection.Exec(query, args...)
	}
	return err
}

func (db *Database) getIntArray(query string, args ...interface{}) ([]int64, error) {
	var arr []int64
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(query, args...)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var element int64
				if err := rows.Scan(&element); err == nil {
					arr = append(arr, element)
				} else {
					break
				}
			}
		}
	}
	return arr, err
}

func (db *Database) GetChats() ([]int64, error) {
	return db.getIntArray(selectChats)
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

func (db *Database) GetAdmins() ([]int64, error) {
	return db.getIntArray(selectAdmins)
}

func (db *Database) GetAdminExist(chat int64) (bool, error) {
	return db.getNotEmpty(existAdmin, chat)
}

func (db *Database) AddAdmin(id int64) error {
	var exist bool
	var err error
	if exist, err = db.GetAdminExist(id); err == nil && !exist {
		err = db.execNoResult(insertAdmin, id)
	}
	return err
}

func (db *Database) DelAdmin(id int64) error {
	return db.execNoResult(delAdmin, id)
}

func (db *Database) GetTorrent(torrent string) (int64, error) {
	var torrentId int64
	var err error
	torrentId = -1
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(selectTorrent, torrent)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&torrentId)
			}
		}
	}
	return torrentId, err
}

func (db *Database) AddTorrent(name string, files []string) (int64, error) {
	var err error
	var id int64
	if err = db.execNoResult(insertOrUpdateTorrent, name); err == nil {
		if id, err = db.GetTorrent(name); err == nil {
			for _, file := range files {
				err = db.execNoResult(insertTorrentFile, id, file)
			}
		}
	}
	return id, err
}

type TorrentFile struct {
	Id      int64
	Torrent int64
	Name    string
	Status  uint8
	EntryId string
}

func (tr *TorrentFile) String() string {
	if tr == nil {
		return "nil"
	}
	return fmt.Sprintf("Id: %d;\tName: %s;\tStatus: %d", tr.Id, tr.Name, tr.Status)
}

func (db *Database) GetTorrentFile(id int64) (TorrentFile, error) {
	var err error
	var file TorrentFile
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(selectTorrentFile, id)
		if err == nil && rows != nil {
			defer rows.Close()
			if rows.Next() {
				file := TorrentFile{}
				err = rows.Scan(&file.Id, &file.Torrent, &file.Name, &file.EntryId, &file.Status)
			}
		}
	}
	return file, err
}

func (db *Database) GetTorrentFilesNotReady() ([]TorrentFile, error) {
	var err error
	var files []TorrentFile
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(selectTorrentFilesNotReady, FileReadyStatus)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				file := TorrentFile{}
				if err = rows.Scan(&file.Id, &file.Torrent, &file.Name, &file.EntryId, &file.Status); err == nil {
					files = append(files, file)
				} else{
					files = []TorrentFile{}
					break
				}
			}
		}
	}
	return files, err
}

func (db *Database) GetTorrentFileIndex(torrent, id int64) (int64, error) {
	var err error
	var index int64
	var tmp []int64
	if tmp, err = db.getIntArray(selectTorrentFileIndex, torrent, id); err == nil && len(tmp) > 0{
		index = tmp[0]
	}
	return index, err
}

func (db *Database) SetTorrentFileStatus(id int64, status uint8) error {
	return db.execNoResult(setTorrentFileStatus, status, id)
}

func (db *Database) SetTorrentFileEntryId(id int64, entryId string) error {
	return db.execNoResult(setTorrentFileEntryId, entryId, id)
}

func (db *Database) getConfigValue(name string) (string, error) {
	var val string
	var err error
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(selectConfig, name)
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

func (db *Database) GetTorrentMeta(id int64) (map[string]string, error) {
	var err error
	meta := make(map[string]string)
	err = db.checkConnection()
	if err == nil {
		var rows *sql.Rows
		rows, err = db.Connection.Query(selectTorrentMeta, id)
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var name, value string
				if err = rows.Scan(&name, &value); err == nil {
					meta[name] = value
				} else {
					break
				}
			}
		}
	}
	return meta, err
}

func (db *Database) AddTorrentMeta(id int64, meta map[string]string) error {
	var err error
	for k, v := range meta {
		if err = db.execNoResult(insertTorrentMeta, id, k, v); err != nil {
			break
		}
	}
	return err
}

func (db *Database) Connect() error {
	var err error
	db.Connection, err = sql.Open(DBDriver, db.ConnectionString)
	if err == nil {
		err = db.checkConnection()
	}
	return err
}

func (db *Database) Close() {
	if db.Connection != nil {
		_ = db.Connection.Close()
	}
}

