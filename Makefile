GO              = go
GOARCH         ?= $(shell $(GO) env GOARCH)
GOOS           ?= $(shell $(GO) env GOOS)


BIN             = $(CURDIR)/bin/$(GOOS)_$(GOARCH)
EXECS           = $(BIN)/exifclean $(BIN)/videothumb $(BIN)/heicthumb \
				  $(BIN)/picloadql $(BIN)/syncAlbum  $(BIN)/checkMedia \
				  $(BIN)/tagAlbum  $(BIN)/exiftool $(BIN)/imagehash \
				  $(BIN)/hashclean
OBJECTS         = sql/*.go cmd/exifclean/*.go cmd/heicthumb/main.go \
				  store/album.go cmd/checkMedia/main.go cmd/tagAlbum/main.go \
                  cmd/picloadql/*.go cmd/videothumb/main.go cmd/imagehash/main.go \
                  store/*.go cmd/syncAlbum/main.go cmd/hashclean/main.go \
				  tools/*.go
CGO_CFLAGS      = $(if $(ACLDIR),-I$(ACLDIR)/inc,)
CGO_LDFLAGS     = $(if $(ACLDIR),-L$(ACLDIR)/lib -ladalnkx,)
CGO_EXT_LDFLAGS = $(if $(ACLDIR),-lsagsmp2 -lsagxts3 -ladazbuf,)
GO_TAGS         = $(if $(ACLDIR),"release adalnk","release")
GO_FLAGS        = $(if $(debug),"-x",) -tags $(GO_TAGS)
PLUGINS         = $(BIN)/plugins/bittools

all: $(EXECS)

plugins: $(PLUGINS)

$(EXECS): $(OBJECTS) ; $(info $(M) building executable $(@:$(BIN)/%=%)…) @ ## Build program binary
	$Q cd $(CURDIR) &&  \
	   CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
		-ldflags '-X $(PACKAGE)/adabas.Version=$(VERSION) -X $(PACKAGE)/adabas.BuildDate=$(DATE)' \
		-o $@ ./cmd/$(@:$(BIN)/%=%)

clean:
	rm -f $(EXECS) *.log

$(PLUGINS):
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
	 -buildmode=plugin \
	 -ldflags '-X $(COPACKAGE).Version=$(VERSION) -X $(COPACKAGE).BuildDate=$(DATE) -s -w' \
	 -o $@.so ./$(@:$(BIN)/%=%)
