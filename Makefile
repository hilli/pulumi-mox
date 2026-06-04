PROVIDER  := mox
VERSION   ?= 0.1.0
PLUGIN    := pulumi-resource-$(PROVIDER)
BIN       := bin/$(PLUGIN)
PKG       := github.com/hilli/pulumi-mox
LDFLAGS   := -X $(PKG)/provider.Version=$(VERSION)

.PHONY: build install_plugin gen_sdk tidy test vet clean

# Build the plugin binary into bin/.
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./provider/cmd/$(PLUGIN)

# Install the freshly-built plugin into the local Pulumi plugin cache so
# examples/ can resolve it without a registry.
install_plugin: build
	pulumi plugin install resource $(PROVIDER) $(VERSION) --file $(BIN)

# Generate language SDKs from the built plugin's schema into sdk/.
# NOTE: do not commit generated SDKs without an explicit decision.
gen_sdk: build
	pulumi package gen-sdk $(BIN)

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin sdk
