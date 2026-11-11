# IceDash build. `make build` checks everything and makes the module image;
# CI runs the same command.
#
#   make build VERSION=v0.2.0      the release build (CI passes the tag)
#   make check                     vet and tests only (works on Windows too)
#   make build BROKEN=1            an image whose service exits at once, for the
#                                  rollback test (acceptance check 9)
#
# The image step needs Linux (mksquashfs); everything before it runs anywhere.

VERSION ?= 0.0.0-dev
ARCH    ?= amd64
BROKEN  ?=
RACE    ?=

MODKIT  := .modkit
ROOTFS  := build/rootfs
WEBOUT  := $(ROOTFS)/usr/share/casaos/www/modules/icedash
LDFLAGS := -s -w -X main.version=$(VERSION:v%=%) $(if $(BROKEN),-X main.broken=1)

.PHONY: build check modkit vet test web-deps web-test rootfs binary web image clean

build: check image

check: modkit vet test web-test

modkit:
	@sh scripts/modkit.sh $(MODKIT)

vet:
	go vet ./...
	GOOS=linux GOARCH=$(ARCH) go vet ./...

test:
	go test $(RACE) -count=1 ./...

web-deps:
	cd web && npm ci --no-audit --no-fund

web-test: web-deps
	cd web && npm run typecheck && npm test

rootfs:
	rm -rf $(ROOTFS)
	mkdir -p build
	cp -R raw $(ROOTFS)

binary: rootfs modkit
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -trimpath -buildvcs=false \
		-ldflags "$(LDFLAGS)" -o $(ROOTFS)/usr/bin/icedash ./cmd/icedash

web: rootfs web-deps
	cd web && npm run build -- --outDir ../$(WEBOUT) --emptyOutDir

image: binary web
	sh $(MODKIT)/sysext/build-sysext.sh icedash $(ROOTFS) $(ARCH) dist $(VERSION)

clean:
	rm -rf build dist
