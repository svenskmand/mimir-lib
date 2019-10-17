# All Go files in the project.
MIMIR-LIB := $(shell find . -not -path "./vendor/*" -name "*.go")
# All Go packages in the project
ALL-PKGS := $(shell go list ./... | grep -v "vendor")
GLIDE-INSTALLED := $(shell which glide)
CURL-INSTALLED := $(shell which curl)

tools: # Run this once to setup the repo, this requires that curl is installed on the system.
ifeq ($(CURL-INSTALLED), )
	@echo "The curl command is not available, ensure it is installed!"
	@exit
endif
ifeq ($(GLIDE-INSTALLED), )
	@curl https://glide.sh/get > glide_install
	@sh glide_install
	@rm -rf glide_install
endif
	@go get golang.org/x/tools/cmd/goimports
	@go get github.com/axw/gocov/gocov
	@go get github.com/AlekSi/gocov-xml
	@go get github.com/matm/gocov-html
	@go get golang.org/x/lint/golint
	@go get golang.org/x/tools/cmd/goimports
	@go get github.com/jstemmer/go-junit-report

all: bins test

bins: dependencies
	@go build $(ALL-PKGS)

dependencies:
	@glide install

lint:
	@gofmt -w -e -s $(MIMIR-LIB)
	@cat /dev/null > lint.log
	@for p in $(ALL-PKGS); do golint $$p >> lint.log ; done
	@cat lint.log
	@go vet $(ALL-PKGS)

test: dependencies lint
	@gocov test $(ALL-PKGS) > test.log
	@cat test.log | gocov report

clean:
	@rm -rf vendor
