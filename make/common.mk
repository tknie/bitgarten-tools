#
# Copyright 2022-2025 Thorsten A. Knieling
#
# SPDX-License-Identifier: Apache-2.0
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#

GOARCH     ?= $(shell $(GO) env GOARCH)
GOOS       ?= $(shell $(GO) env GOOS)
GOEXE      ?= $(shell $(GO) env GOEXE)
GOPATH     ?= $(shell $(GO) env GOPATH)

PKGS        = $(or $(PKG),$(shell cd $(CURDIR) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -v "^vendor/"))
TESTPKGS    = $(shell env GOPATH=$(GOPATH) $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))
CGO_EXT_LDFLAGS =
GO_TAGS     =
GO_FLAGS    = 
GOBIN      ?= $(if $(shell $(GO) env GOBIN),$(shell $(GO) env GOBIN),$(GOPATH)/bin)
GO          = go
GODOC       = godoc
TIMEOUT     = 2000
COPACKAGE   = github.com/tknie/services
BINTESTS    = $(CURDIR)/bin/tests/$(GOOS)_$(GOARCH)
V   = 0
Q   = $(if $(filter 1,$V),,@)
M   = $(shell printf "\033[34;1m▶\033[0m")
UPX := $(shell command -v upx 2> /dev/null)

export TIMEOUT GO CGO_CFLAGS CGO_LDFLAGS GO_FLAGS CGO_EXT_LDFLAGS TESTFILES

.PHONY: all
all: prepare fmt lint lib $(EXECS) test-build

exec: $(EXECS)

lib: $(LIBS)

plugins: $(PLUGINS)

prepare: $(LOGPATH) $(BIN) $(BINTOOLS)
	@echo "Build architecture GOARCH=${GOARCH} GOOS=$(GOOS) suffix=$(GOEXE) network=${WCPHOST} GOFLAGS=$(GO_FLAGS)"
	@echo "GOBIN=$(GOBIN)"
	@mkdir -p $(CURDIR)/logs

$(LIBS): ; $(info $(M) building libraries…) @ # Build program libraries
	$Q cd $(CURDIR) && echo "Building version $(VERSION) && \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
		-buildmode=c-shared \
		-ldflags '-X $(COPACKAGE).Version=$(VERSION) -X $(COPACKAGE).BuildDate=$(DATE) -s -w' \
		-o $(BIN)/$(GOOS)/$@.so $@.go

$(EXECS): $(OBJECTS) ; $(info $(M) building executable $(@:$(BIN)/%=%)…) @ # Build program binary
	$Q cd $(CURDIR) &&  echo "Build date: $(DATE) version: $(VERSION) at $(COPACKAGE).BuildDate $(GOEXE) -> ./cmd/$(@:$(BIN)/%=%)" && \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
		-ldflags '-X $(COPACKAGE).Version=$(RESTVERSION) -X $(COPACKAGE).BuildVersion=$(VERSION) -X $(COPACKAGE).BuildDate=$(DATE)' \
		-o $@$(GOEXE) ./cmd/$(@:$(BIN)/%=%)
#ifdef UPX
#		upx $@$(GOEXE)
#endif

$(PLUGINS): ; $(info $(M) building plugins $(@:$(BIN)/%=%)…) @ # Build plugin binary
	$Q cd $(CURDIR) && \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
	    -buildmode=plugin \
	    -ldflags '-X $(COPACKAGE).Version=$(VERSION) -X $(COPACKAGE).BuildDate=$(DATE) -s -w' \
	    -o $@.so ./$(@:$(BIN)/%=%)

$(LOGPATH):
	@mkdir -p $@

$(BIN):
	@mkdir -p $@
$(BIN)/%: ; $(info $(M) building $(REPOSITORY)…)
	$Q tmp=$$(mktemp -d); \
		(GOPATH=$$tmp CGO_CFLAGS= CGO_LDFLAGS= \
		go get $(REPOSITORY) && cp $$tmp/bin/* $(BIN)/.) || ret=$$?; \
		# (GOPATH=$$tmp go clean -modcache ./...); \
		rm -rf $$tmp ; exit $$ret

$(BINTOOLS):
	@mkdir -p $@
$(BINTOOLS)/%: ; $(info $(M) building tool $(BINTOOLS) on $(REPOSITORY)…)
	$Q tmp=$$(mktemp -d); \
		(GOPATH=$$tmp CGO_CFLAGS= CGO_LDFLAGS= \
		go get $(REPOSITORY) && find $$tmp/bin -type f -exec cp {} $(BINTOOLS)/. \;) || ret=$$?; \
		(GOPATH=$$tmp go clean -modcache ./...); \
		rm -rf $$tmp ; exit $$ret

# Swagger
forceSwagger: cleanSwagger $(GOBIN)/swagger

cleanSwagger:
	rm -f $(GOBIN)/swagger

$(GOBIN)/%: ; $(info $(M) building binary $(TOOL) on $(REPOSITORY)…)
	$Q tmp=$$(mktemp -d); cd $$tmp; \
	CGO_CFLAGS= CGO_LDFLAGS= $(GO) install $(REPOSITORY) || ret=$$?; \
	rm -rf $$tmp ; exit $$ret

# Tools
GOSWAGGER = $(GOBIN)/swagger
$(GOBIN)/swagger: REPOSITORY=github.com/go-swagger/go-swagger/cmd/swagger@latest

GOLINT = $(GOBIN)/golint
$(GOBIN)/golint: REPOSITORY=golang.org/x/lint/golint@latest

GOCOVMERGE = $(GOBIN)/gocovmerge
$(GOBIN)/gocovmerge: REPOSITORY=github.com/wadey/gocovmerge

GOCOV = $(BIN)/gocov
$(BIN)/gocov: REPOSITORY=github.com/axw/gocov/...

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: REPOSITORY=github.com/AlekSi/gocov-xml

GOTESTSUM = $(BINTOOLS)/gotestsum
$(BINTOOLS)/gotestsum: REPOSITORY=gotest.tools/gotestsum

# Tests
$(TESTOUTPUT):
	mkdir $(TESTOUTPUT)

test-build: ; $(info $(M) building $(NAME:%=% )tests…) @ ## Build and start tests
	$Q cd $(CURDIR) && for pkg in $(TESTPKGSDIR); do echo "Build $$pkg in $(CURDIR)"; \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" \
	    TESTFILES=$(TESTFILES) LOGPATH=$(LOGPATH) REFERENCES=$(REFERENCES) \
	    $(GO) test -c -o $(BINTESTS)/$$pkg.test$(GOEXE) $(GO_TAGS) ./$$pkg; done

TEST_TARGETS := test-default test-bench test-short test-json test-verbose test-race test-sanitizer test-single
.PHONY: $(TEST_TARGETS) check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-single:  ARGS=-run $(TESTNAME) ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode
test-json:    ARGS=-json         ## Run tests in json mode
test-race:    ARGS=-race         ## Run tests with race detector
test-sanitizer:  ARGS=-msan      ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt lint ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q cd $(CURDIR) &&  \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" \
	    TESTFILES=$(TESTFILES) LOGPATH=$(LOGPATH) REFERENCES=$(REFERENCES) \
	    $(GO) test -timeout $(TIMEOUT)s -count=1 $(GO_TAGS) $(ARGS) ./...

TEST_XML_TARGETS := test-xml-bench
.PHONY: $(TEST_XML_TARGETS) test-xml
test-xml-bench:     ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks with xml output
$(TEST_XML_TARGETS): NAME=$(MAKECMDGOALS:test-xml-%=%)
$(TEST_XML_TARGETS): test-xml
test-xml: prepare fmt lint $(TESTOUTPUT) | $(GOTESTSUM) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests with xUnit output
	sh $(CURDIR)/scripts/evaluateQueues.sh
	$Q cd $(CURDIR) && 2>&1 TESTFILES=$(TESTFILES)  LOGPATH=$(LOGPATH) \
	    REFERENCES=$(REFERENCES) \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" \
	    ENABLE_DEBUG=$(ENABLE_DEBUG) \
	    $(GOTESTSUM) --junitfile $(TESTOUTPUT)/tests.xml --raw-command -- $(CURDIR)/scripts/test.sh $(ARGS) ||:
	sh $(CURDIR)/scripts/evaluateQueues.sh

COVERAGE_MODE = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOCOVMERGE) $(GOCOV) $(GOCOVXML)
test-coverage: COVERAGE_DIR := $(CURDIR)/test/coverage
test-coverage: fmt lint test-coverage-tools ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)/coverage
	$Q echo "Work on test packages: $(TESTPKGS)"
	$Q cd $(CURDIR) && for pkg in $(TESTPKGS); do echo "Coverage for $$pkg"; \
		TESTFILES=$(TESTFILES) LOGPATH=$(LOGPATH) \
	    REFERENCES=$(REFERENCES) \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" \
	    ENABLE_DEBUG=$(ENABLE_DEBUG) \
		$(GO) test -count=1 \
			-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $$pkg | \
					grep '^$(PACKAGE)/' | grep -v '^$(PACKAGE)/vendor/' | \
					tr '\n' ',')$$pkg \
			-covermode=$(COVERAGE_MODE) -timeout $(TIMEOUT)s $(GO_FLAGS) \
			-coverprofile="$(COVERAGE_DIR)/coverage/`echo $$pkg | tr "/" "-"`.cover" $$pkg ;\
	 done
	$Q echo "Start coverage analysis"
	$Q $(GOCOVMERGE) $(COVERAGE_DIR)/coverage/*.cover > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: lint
lint: | $(GOLINT) ; $(info $(M) running golint…) @ ## Run golint
	$Q cd $(CURDIR) && ret=0 && for pkg in $(PKGS); do \
		$(GOLINT) $$pkg; \
	 done ; exit $$ret

.PHONY: fmt
fmt: ; $(info $(M) running fmt…) @ ## Run go fmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GO) fmt  $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

.PHONY: vet
vet: ; $(info $(M) running vet) @ ## Run go vet on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GO) vet  $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

cleanModules:  ; $(info $(M) cleaning modules) @ ## Build program binary
	echo $(GOPATH)/pkg/mod
ifneq ("$(wildcard $(GOPATH)/pkg/mod)","")
	$Q cd $(CURDIR) &&  \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) clean ./...
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) clean -modcache -cache -testcache
endif

# Misc
.PHONY: clean cleanModules
cleanCommon: cleanModules; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN) $(CURDIR)/pkg $(CURDIR)/logs $(CURDIR)/test $(CURDIR)/log
	@rm -rf test/tests.* test/coverage.*
	@rm -f $(CURDIR)/rest.test $(CURDIR)/*.log $(CURDIR)/*.output

.PHONY: help
help:#
	@echo "Usage:"
	@sed -n 's/:.*##/:/p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /' | egrep -v "sed|grep|awk"

.PHONY: tidy
tidy: ; $(info $(M) tidy up GO mods…) @ ## Run go tidy mods
	GOSUMDB=off CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) mod tidy

.PHONY: doc
doc: ; $(info $(M) running GODOC…) @ ## Run go doc on all source files
	$Q cd $(CURDIR) && echo "Open http://localhost:6060/pkg/github.com/tknie/rest-api/" && \
	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GODOC) -http=:6060 -v -src
#	    CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) doc $(PACKAGE)

.PHONY: vendor-update
vendor-update:
	@echo "Update GO modules"
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) get -u ./...
	$(GO) mod verify
	$(GO) mod tidy
#	GOSUMDB=off CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) get -u ./...

.PHONY: vendor
vendor:
	@echo "Collect GO vendor"
	GOSUMDB=off CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) mod vendor

.PHONY: version
version: ## Show current version in configuration
	@echo $(VERSION)

.PHONY: printVersion
printVersion: version $(RESTEXEC)/cmd/rest
	$(RESTEXEC) version

.PHONY: loadExtPlugins cleanExtPlugins
loadExtPlugins: $(SUBDIRS) ## load external plugin code
	@if [ -r $(PLUGIN_COMPILE_SET) ]; then \
	  mkdir -p $(EXT_PLUGINS); echo $(PLUGINS); \
	  for d in $$(cat $(PLUGIN_COMPILE_SET)); do \
		cd $(EXT_PLUGINS); \
		rep=$$(echo $$d|sed -n 's/,/ /gp'|awk -F' ' '{print $$1}'); \
		dir=$$(echo $$d|sed -n 's/,/ /gp'|awk -F' ' '{print $$2}'); \
		linkdir=$$(echo $$d|sed -n 's/,/ /gp'|awk -F' ' '{print $$3}'); \
		if [ ! -d $$dir ]; then \
		   git clone $$rep $$dir|| ret=$$? ; \
			cd $(PLUGINSSRC); ln -s $(EXT_PLUGINS)/$$dir/$$linkdir $$dir; \
		fi; \
	  done ; \
	fi

cleanExtPlugins: ## clean external plugin code
	@if [ -r $(PLUGIN_COMPILE_SET) ]; then \
	  rm -rf $(EXT_PLUGINS); \
	  for d in $$(cat $(PLUGIN_COMPILE_SET)); do \
		dir=$$(echo $$d|sed -n 's/,/ /gp'|awk -F' ' '{print $$2}'); \
		cd $(PLUGINSSRC); rm -f $$dir; \
	  done ; \
	fi

