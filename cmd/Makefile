default:	ttkvc

FILES = main.go
BUILD_NAME = ttkvc
VERSION = $(shell git tag)
COMMIT = $(shell git rev-parse --short HEAD)
ifdef COMMIT
FLAGS =-ldflags "-X 'sot-te.ch/TtKVC.Version=${COMMIT}'"
endif
ifdef VERSION
FLAGS =-ldflags "-X 'sot-te.ch/TtKVC.Version=${VERSION}'"
endif

ttkvc:	$(FILES)
	go build -a ${FLAGS} -o ${BUILD_NAME} ${FILES}
