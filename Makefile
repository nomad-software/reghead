BINARY := reghead
LDFLAGS := -s -w

.PHONY: build run test vet fuzz screenshot icons appimage macos clean

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

# Makes the icon of the macOS app from the program's icon, within the margin Apple's
# grid gives icons. Run it again whenever the icon changes. It needs ImageMagick.
icons:
	mkdir -p packaging/macos
	magick internal/appicon/appicon.png -filter Lanczos -resize 824x824 -background none -gravity center -extent 1024x1024 packaging/macos/reghead.png

# Packs the program and the Qt libraries it uses into one file that runs without
# installing anything. Releases are made by the GitHub workflow, on an old
# distribution so that they run on more systems.
appimage:
	packaging/appimage.sh

# Builds the macOS app for Apple Silicon, with Qt inside it, in a disk image. It runs
# on a Mac with Homebrew's Qt; releases are made by the GitHub workflow.
macos:
	packaging/macos.sh

clean:
	rm -f $(BINARY)
	rm -rf build
