default:	ttkvc

FILES = main.go
BUILD_NAME = ttkvc
VERSION = $(shell git tag)
ifdef VERSION
FLAGS =-ldflags "-X 'sot-te.ch/TtKVC.Version=${VERSION}'"
endif

ttkvc:	$(FILES)
	go build -a ${FLAGS} -o ${BUILD_NAME} ${FILES}
