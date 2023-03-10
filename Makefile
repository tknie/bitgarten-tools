GO              = go
GOARCH         ?= $(shell $(GO) env GOARCH)
GOOS           ?= $(shell $(GO) env GOOS)


BIN             = $(CURDIR)/bin/$(GOOS)_$(GOARCH)
EXECS           = $(BIN)/album $(BIN)/picload $(BIN)/reader $(BIN)/converter $(BIN)/thumbnail \
	$(BIN)/checkout $(BIN)/checker $(BIN)/cleaner $(BIN)/updoption $(BIN)/htmlload \
	$(BIN)/picloadql
OBJECTS         = album/main.go picload/main.go reader/main.go sql/*.go converter/main.go \
   store/picture.go store/store.go thumbnail/main.go checkout/main.go updoption/main.go \
   store/adabas.go store/worker.go store/album.go checker/main.go cleaner/main.go \
   picloadql/*.go sql/*.go store/*.go
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
		-o $@ ./$(@:$(BIN)/%=%)

clean:
	rm -f $(EXECS)
