APP_NAME := AutotaskMenu
BUNDLE := build/$(APP_NAME).app
EXEC := $(BUNDLE)/Contents/MacOS/$(APP_NAME)
SOURCES := $(wildcard Sources/AutotaskMenu/*.swift)
RESOURCES := $(wildcard Resources/*)
SWIFTC := /usr/bin/xcrun swiftc

.PHONY: build run clean

build: $(EXEC)

$(EXEC): $(SOURCES) $(RESOURCES) Info.plist
	rm -rf $(BUNDLE)
	mkdir -p $(BUNDLE)/Contents/MacOS $(BUNDLE)/Contents/Resources
	cp Info.plist $(BUNDLE)/Contents/Info.plist
	cp Resources/* $(BUNDLE)/Contents/Resources/
	$(SWIFTC) -parse-as-library -O -target arm64-apple-macos13 \
		-framework SwiftUI -framework AppKit \
		-o $(EXEC) $(SOURCES)

run: build
	open $(BUNDLE)

clean:
	rm -rf build
