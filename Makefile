BINARY := reghead
LDFLAGS := -s -w

.PHONY: build run test vet fuzz screenshot appimage clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test -race -timeout 5m ./...

vet:
	go vet ./...

# The first run recompiles the Qt bindings with coverage, which takes minutes.
fuzz:
	go test ./internal/engine -run '^$$' -fuzz FuzzEngines -fuzztime 60s
	go test ./internal/engine -run '^$$' -fuzz FuzzReplacement -fuzztime 30s

# Renders the window offscreen, in the dark theme, into docs/screenshot.png.
screenshot:
	mkdir -p docs
	REGHEAD_THEME=dark REGHEAD_SCREENSHOT=$(CURDIR)/docs/screenshot.png go test -count=1 -run '^TestScreenshot$$' ./internal/ui/

# Packs the program and the Qt libraries it uses into one file that runs without
# installing anything, put together in /tmp/reghead-build and left here as
# RegHead-<version>-x86_64.AppImage. Releases are made by the GitHub workflow, on
# an old distribution so that they run on more systems.
appimage:
	packaging/appimage.sh

clean:
	rm -f $(BINARY) RegHead-*.AppImage
	rm -rf /tmp/reghead-build
