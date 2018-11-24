CMD_FILES = $(wildcard cmd/wal-g/*.go)
PKG_FILES = $(wildcard *.go)
PKG := github.com/wal-g/wal-g

.PHONY : fmt test install all clean alpine

ifdef GOTAGS
override GOTAGS := -tags $(GOTAGS)
endif

test: cmd/wal-g/wal-g
	go list ./... | grep -v 'vendor/' | xargs go vet
	go test -v ./walg_test/
	go test -v ./walparser/

fmt: $(CMD_FILES) $(PKG_FILES)
	gofmt -s -w $(CMD_FILES) $(PKG_FILES)

all: cmd/wal-g/wal-g

install:
	(cd cmd/wal-g && go install)

clean:
	rm -rf extracted compressed $(filter-out $(wildcard *.go), $(wildcard data*))
	go clean
	(cd cmd/wal-g && go clean)

cmd/wal-g/wal-g: $(CMD_FILES) $(PKG_FILES)
	(cd cmd/wal-g && go build $(GOTAGS) -ldflags "-s -w -X main.BuildDate=`date -u +%Y.%m.%d_%H:%M:%S` -X main.GitRevision=`git rev-parse --short HEAD` -X main.WalgVersion=`git tag -l --points-at HEAD`")

alpine: $(CMD_FILES) $(PKG_FILES)
	docker run                                                              \
	    --rm                                                                \
	    -v /tmp:/.cache                                                     \
	    -v "$$(pwd):/go/src/$(PKG)"                                         \
	    -w /go/src/$(PKG)                                                   \
	    -e GOOS=linux                                                       \
	    -e GOARCH=amd64                                                     \
	    golang:1.11.2-alpine                                                \
	    ./build-alpine.sh
    # sudo chown $$(id -u):$$(id -g) cmd/wal-g/wal-g
	# sudo rm -rf .brotli.tmp
	# sudo rm -rf ./vendor/github.com/google/brotli/dist
