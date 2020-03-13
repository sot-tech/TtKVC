module sot-te.ch/TtKVC

go 1.13

require (
	github.com/hekmon/transmissionrpc v0.1.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/zeebo/bencode v1.0.0
	sot-te.ch/HTExtractor v0.1.0
	sot-te.ch/MTHelper v0.1.2
)

replace sot-te.ch/MTHelper => ../MTHelper

replace sot-te.ch/HTExtractor => ../HTExtractor
