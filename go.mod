module sot-te.ch/TtKVC

go 1.13

require (
	github.com/hekmon/transmissionrpc v0.1.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/sot-tech/telegram-bot-api v0.0.0-00010101000000-000000000000
	github.com/zeebo/bencode v1.0.0
	sot-te.ch/HTExtractor v0.0.0
	sot-te.ch/TGHelper v0.1.9
)

replace sot-te.ch/TGHelper => ../TGHelper

replace sot-te.ch/HTExtractor => ../HTExtractor

replace github.com/sot-tech/telegram-bot-api => ../telegram-bot-api
