default: containerized_build

LCACHE_VERSION ?= 1.0

export GOPATH=$(CURDIR)/
export GOBIN=$(CURDIR)/.output/bin
export GOCACHE=$(CURDIR)/.output/cache
export LCACHE_VERSION

clean:
	@rm -rf .temp .output path src

fmt:
	@go fmt .
	@go fmt ./src/lcache

vendor:
	@mkdir -p path/src
	@# clear our previous vendoring
	@find ./path/src/ -mindepth 1 -maxdepth 1 -type d | xargs rm -rf

	@rm -f $(CURDIR)/path/src/lcache
	@ln -s $(CURDIR) $(CURDIR)/path/src/lcache

	go get -t -d ./...

	@# Remove submodule git directories
	@find ./path -mindepth 2 -name ".git" -type d | xargs rm -rf

test:
	@go test .

containerized_build:
	docker run \
		--rm \
		-v "$(CURDIR)":"/srv/build":rw \
		-u "$(shell id -u $(whoami)):$(shell id -g $(whoami))" \
		-e LCACHE_VERSION=$(LCACHE_VERSION) \
		golang:1.14 \
		bash -c \
		"cd /srv/build; make test"