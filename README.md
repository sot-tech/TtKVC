# Torrent & Telegram & Kaltura Video Connector
Torrent tracker site release watcher, uploader to kaltura video platform and telegram notifier.
Can:

 - Watch tracker site for new torrent releases (it enumerates serial IDs and searches for torrent-like data)
 - Upload torrent to transmission server
 - Determine pretty name of release from tracker site
 - Upload video to kaltura platform
 - Upload converted video to telegram

Uses:

 - [go-logging (BSD-3)](https://github.com/op/go-logging)
 - [Go Bencode (MIT)](https://github.com/zeebo/bencode)
 - [Go-SQLLite3 (MIT)](https://github.com/mattn/go-sqlite3)
 - [transmissionrpc (MIT)](https://github.com/hekmon/transmissionrpc)
 - sot-te.ch/GoMTHelper (BSD-3)
 - sot-te.ch/GoHTExtractor (BSD-3)
 
# Usage
## Quick start
1. Compile sources from `cmd` with `make`
2. Copy example config and database from `conf` to place you want
3. Rename and modify `example.json` with your values
4. Run

```
ttkvc -c /etc/ttkvc.json
```

## Configuration

 - log - file to store error and warning messages
	- file - string - file to store messages
	- level - string - minimum log level to store (DEBUG, NOTICE, INFO, WARNING, ERROR)
 - crawler
	- baseurl - string - base url (`http://site.local`)
	- contexturl - string - torrent context respectively to `baseurl` (`/catalog/%d`, `%d` - is the place to insert id)
	- threshold - uint - number to id's to check in one try. If current id is 1000 and `threshold` set to 3, observer will check 1000, 1001, 1002
	- delay - uint - minimum delay between two checks, real delay is random between value and 2*value
	- reloaddelay - uint - if torrent download success, retry download after some seconds to ensure, that torrent has already proceed by tracker
	- ignoreregexp - string - filename regexp to **not** upload to kaltura
	- metaactions - list of actions to extract meta info release (see GoHTExtractor readme)
 - transmission
	- host - string - hostname of transmission server
	- port - uint - port that transmission server listens
	- login - string
	- password - string
	- path - string - path of transmission server to download torrent files
	- encryption - bool - use encryption to connect to transmission
	- trackers - string array - couple of other trackers URLs to append to torrent
 - kaltura
	- url - string - base url to kaltura
    - partnerid - uint
    - userid - string - kaltura user login
    - secret - string - kaltura user secret
    - watchpath - string - to watch for downloaded files
    - tags - map of string-boolean - keys of meta info, extracted with `metaactions` to create tags in kaltura, if set to true - try to split comma-separated string and process individually
    - entryname - string - template of entry name, if result string is empty - fallback to file name. Possible placeholders:
        - `{{.meta.*}}` - value from extracted meta (instead of `*`)
        - `{{.index}}` - file order in torrent (sorted by file name)
        - `{{.id}}` - unique file id in DB
        - `{{.name}}` - file name
 - telegram
	- apiid - int - API ID received from [telegram](https://my.telegram.org/apps)
    - apihash - string - API HASH received from [telegram](https://my.telegram.org/apps)
    - bottoken - string
    - dbpath - string - TDLib's DB path (used to store session data)
    - filestorepath - string - TDLib's file store path (can be temporary)
    - otpseed - string - base32 encoded random bytes to init TOTP (for admin auth)
    - msg
        - error - string - message prepended to error
        - auth - string - response to `/setadmin` or `/rmadmin` if unauthorized (OTP invalid)
        - cmds - command responses
            - start - string - response to `/start` command
        	- attach - string - response to `/attach` command if succeeded
        	- detach - string - response to `/detach` command if succeeded
        	- setadmin - string - response to `/setadmin` command if succeeded
        	- rmadmin - string - response to `/rmadmin` command if succeeded
        	- unknown - string - response to unsupported command
        - state - string - response template to `/state` command. Possible placeholders:
        	- `{{.admin}}` - is this chat has admin privilegies
        	- `{{.watch}}` - is this chat subscribed to announces
        	- `{{.index}}` - next check index
        	- `{{.files}}` - list of pending files
        	- `{{.version}}` - version of the app
        - videoignored - string - message template when video uploaded to kaltura, but **won't** be uploaded to telegram. Possible placeholders:
            - `{{.name}}` - file name
            - `{{.ignorecmd}}` - command to force upload to telegram 
        - videoforced - string - message template when video uploaded to kaltura, and **will** be uploaded to telegram. Placeholders same as previous.
        - kupload - string  - message template when video entry created in kaltura. Possible placeholders:
            - `{{.name}}` - file name
            - `{{.id}}` - kaltura media entry id
        - tupload - string - message template for telegram video caption. Possible placeholders:
            - `{{.meta.*}}` - value from extracted meta (instead of `*`)
            - `{{.index}}` - file order in torrent (sorted by file name)
            - `{{.tags}}` - formatted tags from kaltura
    - video
        - upload - bool - automatically upload converted videos to telegram
        - sequential - bool - video won't be uploaded to telegram until previous (by name order) videos from torrent not in ready state
        - temppath - string - temp path to store video, downloaded from kaltura
 - db
	- connection - string - path to db

## Admins
Administrators are chats, that receive messages about kaltura uploads, and can disable or enable upload video to telegram (for particular video).
`/switchignore_{id}` - switch status of file. If particular file set to not upload - it will be uploaded to Telegram and vice versa. 
_NB: id - is identifier in DB._

`/forceupload {id}` -  forcibly upload file with provided id, even if file names inside torrent matches with `crawler.ignoreregexp`. 
_NB: id - is offset respectively to `crawler.contexturl`._

To become admin, chat should call `/setadmin 123456` in telegram, where 123456 - is an OTP, seeded by `adminotpseed`,
to revoke admin call `/rmadmin 123456`.

