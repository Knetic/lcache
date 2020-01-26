default: test

LCACHE_VERSION ?= 1.0

export GOPATH=$(CURDIR)/
export GOBIN=$(CURDIR)/.temp/
export LCACHE_VERSION

clean:
	@rm -rf ./.output/
	@rm -rf ./.temp/

fmt:
	@go fmt .
	@go fmt ./src/lcache

vendor:
	@mkdir -p path/src
	@# clear our previous vendoring
	@find ./path/src/ -mindepth 1 -maxdepth 1 -type d | xargs rm -rf

	@rm -f $(CURDIR)/path/src/lcache
	@ln -s $(CURDIR) $(CURDIR)/path/src/lcache

	go get -d ./...

	@# Remove submodule git directories
	@find ./path -mindepth 2 -name ".git" -type d | xargs rm -rf

test:
	@go test .