VERSION := $(shell git describe --always --long --dirty)
BUILDTIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
RELEASE_VERSION := $(shell git describe --abbrev=0)

GOLDFLAGS += -X main.version=$(VERSION)
GOLDFLAGS += -X main.buildTime=$(BUILDTIME)
GOFLAGS = -ldflags "$(GOLDFLAGS)"
BINARY_NAME=AxisBodyWornSwiftServiceExample
BINARY_LINUX_AMD64=$(BINARY_NAME)_linux-amd64
BINARY_LINUX_ARM64=$(BINARY_NAME)_linux-arm64
BINARY_WINDOWS=$(BINARY_NAME)_windows_amd64.exe
BINARY_DARWIN_AMD64=$(BINARY_NAME)_darwin-amd64
GPS_BINARY_NAME=GNSSViewerExample
GPS_BINARY_LINUX_AMD64=$(GPS_BINARY_NAME)_linux-amd64
GPS_BINARY_LINUX_ARM64=$(GPS_BINARY_NAME)_linux-arm64
GPS_BINARY_WINDOWS=$(GPS_BINARY_NAME)_windows_amd64.exe
GPS_BINARY_DARWIN_AMD64=$(GPS_BINARY_NAME)_darwin-amd64

ZIP_CONTENT=$(BINARY_WINDOWS) \
			$(BINARY_LINUX_AMD64) \
			$(BINARY_LINUX_ARM64) \
			$(BINARY_DARWIN_AMD64) \
			$(GPS_BINARY_WINDOWS) \
			$(GPS_BINARY_LINUX_AMD64) \
			$(GPS_BINARY_LINUX_ARM64) \
			$(GPS_BINARY_DARWIN_AMD64) \
			cmd/gnss_viewer/main.go \
			cmd/gnss_viewer/gps_converter.go \
			cmd/gnss_viewer/index.html \
			cmd/media-storage-service/main.go \
			server/capability.go \
			server/certificate_test.go \
			server/configure.go \
			server/logger.go \
			server/middleware.go \
			server/server_test.go \
			server/server.go \
			CODEOWNERS \
			CONTRIBUTING.md \
			decrypt_file.sh \
			EXAMPLE.md \
			go.mod \
			go.sum \
			LICENSE \
			Makefile \
			NEWS \
			play_encrypted \
			README.md \
			VERSION

build:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_LINUX_AMD64) -v $(GOFLAGS) ./cmd/media-storage-service/
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_LINUX_ARM64) -v $(GOFLAGS) ./cmd/media-storage-service/
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_DARWIN_AMD64) -v $(GOFLAGS) ./cmd/media-storage-service/
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_WINDOWS) -v $(GOFLAGS) ./cmd/media-storage-service/
	GOOS=linux GOARCH=amd64 go build -o $(GPS_BINARY_LINUX_AMD64) -v $(GOFLAGS) ./cmd/gnss_viewer/
	GOOS=linux GOARCH=arm64 go build -o $(GPS_BINARY_LINUX_ARM64) -v $(GOFLAGS) ./cmd/gnss_viewer/
	GOOS=darwin GOARCH=amd64 go build -o $(GPS_BINARY_DARWIN_AMD64) -v $(GOFLAGS) ./cmd/gnss_viewer/
	GOOS=windows GOARCH=amd64 go build -o $(GPS_BINARY_WINDOWS) -v $(GOFLAGS) ./cmd/gnss_viewer/

clean:
	go clean
	rm $(BINARY_LINUX_AMD64)
	rm $(BINARY_LINUX_ARM64)
	rm $(BINARY_DARWIN_AMD64)
	rm $(BINARY_WINDOWS)
	rm $(GPS_BINARY_LINUX_AMD64)
	rm $(GPS_BINARY_LINUX_ARM64)
	rm $(GPS_BINARY_DARWIN_AMD64)
	rm $(GPS_BINARY_WINDOWS)

zip: build
	echo ${RELEASE_VERSION} > VERSION
	zip ${BINARY_NAME}_${RELEASE_VERSION}.zip ${ZIP_CONTENT}
	rm VERSION
