PROVIDER  := mox
VERSION   ?= 0.1.0
PLUGIN    := pulumi-resource-$(PROVIDER)
BIN       := bin/$(PLUGIN)
PKG       := github.com/hilli/pulumi-mox
LDFLAGS   := -X $(PKG)/provider.Version=$(VERSION)

.PHONY: build install_plugin gen_sdk tidy test vet clean e2e

# Build the plugin binary into bin/.
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./provider/cmd/$(PLUGIN)

# Install the freshly-built plugin into the local Pulumi plugin cache so
# examples/ can resolve it without a registry.
install_plugin: build
	pulumi plugin install resource $(PROVIDER) $(VERSION) --file $(BIN) --reinstall

# Generate language SDKs from the built plugin's schema into sdk/.
# NOTE: do not commit generated SDKs without an explicit decision.
gen_sdk: build
	pulumi package gen-sdk $(BIN)

# Live end-to-end test against a local `mox localserve`. Starts an ephemeral
# mox (or reuses one with MOX_E2E_REUSE=1), runs the examples/yaml stack, then
# tears everything down. See examples/yaml/e2e.sh for env knobs.
e2e: install_plugin
	./examples/yaml/e2e.sh

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin sdk
