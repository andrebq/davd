.PHONY: build run test tidy
.SILENT: run-add-user

include Environment.mk

build: test
	mkdir -p dist
	go build -o dist/davd .

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t andrebq/davd:latest .

docker-push: docker-build
	docker push andrebq/davd:latest

docker-quick-build:
	docker buildx build --platform linux/arm64 -t andrebq/davd:latest .

test: tidy
	true

tidy:
	go fmt ./...
	go mod tidy

run:
	mkdir -p $(localfiles)
	mkdir -p $(localfiles)/scratch
	mkdir -p $(argRootDir)
	mkdir -p $(DAVD_SERVER_CONFIG_DIR)

	DAVD_ADDR=127.0.0.1 \
		DAVD_PORT=8080 \
		DAVD_ROOT_DIR=$(argRootDir) \
		DAVD_ADMIN_TOKEN=$(DAVD_ADMIN_TOKEN) \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		DAVD_DYNBIND_SCRATCH=scratch:$(localfiles)/scratch \
		./dist/davd server run

argUsername?=test
argPassword?=test
run-add-user:
	echo $(argPassword) | \
		DAVD_SERVER_CONFIG_DIR=$(DAVD_SERVER_CONFIG_DIR) \
		./dist/davd auth user add --name=$(argUsername)

run-filestash-demo:
	mkdir -p $(localfiles)/filestash
	$(docker) run --rm -ti -v $(localfiles)/filestash:/app/data/state/ -p 8334:8334 machines/filestash