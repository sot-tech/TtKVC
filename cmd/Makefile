default:	ttkvc

FILES = main.go ../*.go
BUILD_NAME = ttkvc
VERSION = $(shell git tag)
COMMIT = $(shell git rev-parse --short HEAD)
ifeq ($(COMMIT),)
ifdef VERSION
FLAGS =-ldflags "-X 'sot-te.ch/TtKVC.Version=${VERSION}'"
endif
else
FLAGS =-ldflags "-X 'sot-te.ch/TtKVC.Version=${COMMIT}'"
endif

ttkvc:	$(FILES)
	go build -a ${FLAGS} -o ${BUILD_NAME} ${FILES}
