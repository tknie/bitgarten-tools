GO              = go
GOARCH         ?= $(shell $(GO) env GOARCH)
GOOS           ?= $(shell $(GO) env GOOS)


BIN             = $(CURDIR)/bin/$(GOOS)_$(GOARCH)
EXECS           = $(BIN)/checker $(BIN)/exifclean $(BIN)/videothumb $(BIN)/thumbnail \
				  $(BIN)/picloadql $(BIN)/syncAlbum $(BIN)/checkMedia $(BIN)/updoption
OBJECTS         = sql/*.go store/picture.go store/store.go cmd/exifclean/*.go \
				  store/adabas.go store/album.go cmd/checkMedia/main.go \
                  cmd/checker/main.go cmd/picloadql/*.go cmd/videothumb/main.go \
                  sql/*.go store/*.go cmd/syncAlbum/main.go cmd/updoption/main.go
CGO_CFLAGS      = $(if $(ACLDIR),-I$(ACLDIR)/inc,)
CGO_LDFLAGS     = $(if $(ACLDIR),-L$(ACLDIR)/lib -ladalnkx,)
CGO_EXT_LDFLAGS = $(if $(ACLDIR),-lsagsmp2 -lsagxts3 -ladazbuf,)
GO_TAGS         = $(if $(ACLDIR),"release adalnk","release")
GO_FLAGS        = $(if $(debug),"-x",) -tags $(GO_TAGS)

all: $(EXECS)

$(EXECS): $(OBJECTS) ; $(info $(M) building executable $(@:$(BIN)/%=%)…) @ ## Build program binary
	$Q cd $(CURDIR) &&  \
	   CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS) $(CGO_EXT_LDFLAGS)" $(GO) build $(GO_FLAGS) \
		-ldflags '-X $(PACKAGE)/adabas.Version=$(VERSION) -X $(PACKAGE)/adabas.BuildDate=$(DATE)' \
		-o $@ ./cmd/$(@:$(BIN)/%=%)

clean:
	rm -f $(EXECS) *.log
