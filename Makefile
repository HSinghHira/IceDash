# IceDash build. `make build` checks everything and makes the ZimaOS module
# image and the CasaOS packages; CI runs the same command.
#
#   make build VERSION=v0.2.0      the release build (CI passes the tag)
#   make check                     vet and tests only (works on Windows too)
#   make build BROKEN=1            an image whose service exits at once, for the
#                                  rollback test (acceptance check 9)
#
# The image step needs Linux (mksquashfs); everything before it runs anywhere.
# `make casaos ARCH=arm64` makes one CasaOS package (PLAN.md, Phase 3a).

VERSION ?= 0.0.0-dev
ARCH    ?= amd64
BROKEN  ?=
RACE    ?=

MODKIT  := .modkit
ROOTFS  := build/rootfs
WEBOUT  := $(ROOTFS)/usr/share/casaos/www/modules/icedash
LDFLAGS := -s -w -X main.version=$(VERSION:v%=%) $(if $(BROKEN),-X main.broken=1)

CASAOS  := build/casaos-$(ARCH)

.PHONY: build check modkit vet test web-deps web-test rootfs binary web image casaos casaos-all preview clean

build: check image casaos-all

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
	chmod 0755 $(ROOTFS)/usr/bin/icedash  # a Windows build or copy loses the flag (measured on the box)

web: rootfs web-deps
	cd web && npm run build -- --outDir ../$(WEBOUT) --emptyOutDir

image: binary web
	sh $(MODKIT)/sysext/build-sysext.sh icedash $(ROOTFS) $(ARCH) dist $(VERSION)

# The CasaOS package: the program, the web app, the unit and the scripts, as
# dist/icedash-casaos-<arch>.tar.gz with its .sha256, plus the installer.
casaos: web modkit
	rm -rf $(CASAOS)
	mkdir -p $(CASAOS) dist
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -trimpath -buildvcs=false \
		-ldflags "$(LDFLAGS)" -o $(CASAOS)/icedash ./cmd/icedash
	cp -R $(WEBOUT) $(CASAOS)/web
	cp packaging/casaos/icedash.service packaging/casaos/uninstall.sh $(CASAOS)/
	echo "$(VERSION:v%=%)" > $(CASAOS)/VERSION
	chmod 0755 $(CASAOS)/icedash $(CASAOS)/uninstall.sh
	tar -C $(CASAOS) --owner=0 --group=0 --numeric-owner -czf dist/icedash-casaos-$(ARCH).tar.gz .
	cd dist && sha256sum icedash-casaos-$(ARCH).tar.gz > icedash-casaos-$(ARCH).tar.gz.sha256
	cp packaging/casaos/install-casaos.sh dist/install-casaos.sh

casaos-all:
	$(MAKE) casaos ARCH=amd64
	$(MAKE) casaos ARCH=arm64

# See the web app on this PC with sample data shaped like a real box; nothing
# is built or pushed. Open http://127.0.0.1:5199/modules/icedash/ (it opens by
# itself). Add ?demo=icestore, setup, clash, installing, menu, creds or first
# to open a screen.
preview: web-deps
	cd web && npm run mock

clean:
	rm -rf build dist
