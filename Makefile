SHELL=/usr/bin/env bash

PROJECT:=github.com/dafsic/hunter

BINDIR     := $(CURDIR)/bin
BINNAME    ?= $(notdir $(PROJECT))
GO_VERSION := $(shell go version)
BUILD_TIME := $(shell date +%Y%m%d%H%M%S)
#BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S%z)

# go option
PKG         := ./...
TESTS       := .
TESTFLAGS   :=
CGO_ENABLED ?= 0
GO_LDFLAGS += -s -w

# Rebuild the binary if any of these files change
SRC := $(shell find . -type f -name '*.go' -print) go.mod go.sum

COMMIT_HASH := $(shell git rev-parse --short=8 HEAD || echo unknown)
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD)
GIT_DIRTY   := $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
GIT_TAG     := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
#GIT_TAG     := $(shell git describe --tags `git rev-list --tags --max-count=1`)

GO_LDFLAGS += -X '$(PROJECT)/version.build_time=$(BUILD_TIME)'
GO_LDFLAGS += -X '$(PROJECT)/version.go_version=$(GO_VERSION)'
GO_LDFLAGS += -X '$(PROJECT)/version.commit_hash=$(COMMIT_HASH)'
GO_LDFLAGS += -X '$(PROJECT)/version.git_branch=$(GIT_BRANCH)'
GO_LDFLAGS += -X '$(PROJECT)/version.version=$(GIT_TAG)'
GO_LDFLAGS += -X '$(PROJECT)/version.git_tree_state=$(GIT_DIRTY)'

.PHONY: default
default: compile

# --------------------------------------------------------------------------------
# compile

.PHONY: check compile

check: ## Check working tree is clean or not
ifneq ($(shell git status -s),)
	$(error You must run git commit)
endif

compile: $(BINDIR)/$(BINNAME) ## Compile the binary

$(BINDIR)/$(BINNAME): $(SRC)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BINDIR)/$(BINNAME) ./cmd/hunter

# --------------------------------------------------------------------------------
# build image

.PHONY: build
build: ## Build the docker image
	@docker build -t 192.168.70.202:32373/registry/mgs/cloud/platform/hunter:$(BUILD_TIME)-$(GIT_BRANCH) .
	@echo 192.168.70.202:32373/registry/mgs/cloud/platform/hunter:$(BUILD_TIME)-$(GIT_BRANCH) > ./image.txt

# --------------------------------------------------------------------------------
# push image

.PHONY: push
push: ## Push the docker image to registry
	@docker push `cat ./image.txt` > /dev/null
	@echo pushed image: `cat ./image.txt`

# --------------------------------------------------------------------------------
# swagger

.PHONY: swagger
swag: ## Generate swagger docs
	@swag init -g cmd/hunter/main.go

# --------------------------------------------------------------------------------
# test

TESTFLAGS += -v
#TESTFLAGS += -race

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo
	@echo "==> Running unit tests <=="
	GO111MODULE=on go test $(TESTFLAGS) -run $(TESTS) $(PKG) 

.PHONY: test-coverage
test-coverage: ## Run unit tests with coverage
	@echo
	@echo "==> Running unit tests with coverage <=="
	@ ./scripts/coverage.sh

# --------------------------------------------------------------------------------
# clean

.PHONY: clean
clean: ## Remove previous build
	@go clean
	@rm -f $(BINDIR)/$(BINNAME)
	@docker rmi `cat ./image.txt` &> /dev/null || true


# --------------------------------------------------------------------------------
# help

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'