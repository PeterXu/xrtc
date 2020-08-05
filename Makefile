OS := $(shell uname)
NS = peterxu
VERSION ?= latest
TARGET_IMG = $(NS)/docker-xrtc
BUILD_IMG = $(NS)/docker-xrtc-build

HOST_ADDR ?= "192.168.2.3"
PKG_CFG ?= "/usr/local/lib/pkgconfig"

all: build

gen:
	@protoc --go_out=src proto/route.proto -Iproto
	@protoc --go_out=src proto/rest.proto -Iproto

build:
	@PKG_CONFIG_PATH=$(PKG_CFG) go build -ldflags "-s -w"

clean:
	@go clean

check:
	@PKG_CONFIG_PATH=$(PKG_CFG) go get -u

run: build
	@go run main.go

prepare:
	@mkdir -p /tmp/etc/
	@cp scripts/routes.yml  /tmp/etc/
	@cp -f scripts/certs/cert.pem /tmp/etc/cert.pem
	@cp -f scripts/certs/key.pem /tmp/etc/cert.key
	@tar xf scripts/GeoLite2-City.mmdb.tgz -C /tmp/etc/

## docker

docker: build
	docker build -t $(TARGET_IMG):$(VERSION) -f scripts/Dockerfile .

deploy: 
	@export candidate_host_ip=$(HOST_ADDR) && \
		docker-compose -f scripts/docker-compose.yml up -d

deploy-mac:
	@export candidate_host_ip=$(HOST_ADDR) && \
		docker-compose -f scripts/docker-compose.cross.yml up -d
	@docker logs -f xrtc-proxy

docker-pull:
	@docker pull $(TARGET_IMG):latest
	@docker pull $(BUILD_IMG):latest

docker-build:
	@test "$(OS)" = "Linux"
	@(cp -rf /usr/local/include scripts/include)
	@(cp -rf /usr/local/lib scripts/lib)
	@(docker build -t $(BUILD_IMG):$(VERSION) -f scripts/Dockerfile.build build)
	@(rm -rf scripts/include scripts/lib)

docker-test:
	@docker run -v $(GOPATH):/gopath -v $(shell pwd):/gobuild -it --rm $(BUILD_IMG) make test

docker-mac:
	@docker run -v $(GOPATH):/gopath -v $(shell pwd):/gobuild -it --rm $(BUILD_IMG) make
	@mv -f gobuild xrtc.gen
	@docker build -t $(TARGET_IMG):cross -f scripts/Dockerfile.cross .

