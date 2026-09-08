VERSION := $(strip $(shell cat VERSION))
REVISION ?= $(strip $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown))
GSM_IMAGE ?= gsm-system:$(VERSION)
GSM_IMMUTABLE_IMAGE ?= gsm-system:$(VERSION)-$(REVISION)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.revision=$(REVISION)

.PHONY: all build test vet clean linux docker docker-release docker-contract

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/gsm-system ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin

# Cross-compile for the Ubuntu SDR server. / 为 Ubuntu SDR 服务器交叉编译。
linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/gsm-system-linux-amd64 ./cmd/server

# Build the immutable image without starting a container. / 仅构建不可变镜像。
docker:
	GSM_IMAGE="$(GSM_IMMUTABLE_IMAGE)" GSM_VERSION="$(VERSION)" GSM_REVISION="$(REVISION)" docker compose -p gsm-system-live -f deploy/docker/docker-compose.yml build

# Build, deploy, verify, then publish the mutable release alias.
# 构建部署并验证后，才发布可变版本别名。
docker-release:
	GSM_VERSION="$(VERSION)" GSM_REVISION="$(REVISION)" ./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live

docker-contract:
	sh scripts/tests/test_build_contract.sh
