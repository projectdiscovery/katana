# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test
GOFLAGS := -v
# This should be disabled if the binary uses pprof
LDFLAGS := -s -w

ifneq ($(shell go env GOOS),darwin)
LDFLAGS := -extldflags "-static"
endif

all: build
build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "katana" cmd/katana/main.go
test:
	$(GOTEST) $(GOFLAGS) ./...
integration:
	cd integration_tests; bash run.sh cd ..
tidy:
	$(GOMOD) tidy
