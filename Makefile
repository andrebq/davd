.PHONY: build run test tidy
.SILENT: run-add-user

include Environment.mk

IMAGE_REPO?=andrebq/davd
imageLabel?=latest
IMAGE_FULL_NAME?=$(IMAGE_REPO):$(imageLabel)

build: test
	mkdir -p dist
	go build -o dist/davd .

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE_FULL_NAME) .

docker-push: docker-build
	docker push $(IMAGE_FULL_NAME)

docker-quick-build:
	docker buildx build --platform linux/arm64 -t $(IMAGE_FULL_NAME) .

docker-daily-push:
	$(MAKE) docker-push imageLabel=$(shell date '+%Y-%m-%d')

test: tidy
	true

tidy:
	go fmt ./...
	go mod tidy

run: build
	mkdir -p $(localfiles)
	mkdir -p $(localfiles)/scratch
	mkdir -p $(argRootDir)
	mkdir -p $(DAVD_SERVER_CONFIG_DIR)

	DAVD_ADDR=127.0.0.1 \
		DAVD_PORT=8080 \
		DAVD_DEBUG=1 \
		DAVD_ROOT_DIR=$(argRootDir) \
		DAVD_ADMIN_TOKEN=$(DAVD_ADMIN_TOKEN) \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_DYNBIND_SCRATCH=scratch:$(localfiles)/scratch \
		DAVD_DYNBIND_PWD=pwd:$(PWD) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd server run

argUsername?=test
argPassword?=test
argCanWrite?=-w
argPermission?=/binds/
run-add-user:
	echo $(argPassword) | \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user add --name=$(argUsername)

run-add-permission:
	DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user update-permission --name=$(argUsername) -p $(argPermission) $(argCanWrite)

run-add-scratch-user:
	echo scratch | \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user add --name=scratch
	DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user update-permission --name=scratch -p /binds/scratch

run-add-pwd-user:
	echo $(argPassword) | \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user add --name=pwd
	DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user update-permission --name=pwd -p /pwd/ -w
	DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_SEED_KEY=$(DAVD_SEED_KEY) \
		./dist/davd auth user list-permissions --name=pwd

run-filestash-demo:
	mkdir -p $(localfiles)/filestash
	$(docker) run --rm -ti -v $(localfiles)/filestash:/app/data/state/ -p 8334:8334 machines/filestash