VERSION ?= $(shell git describe --tags 2>/dev/null || echo "0.1.0-dev")
DIST := dist
DEB_ROOT := $(DIST)/proxmaid_$(VERSION)

.PHONY: all build build-api build-ui test clean package install

all: build

build: build-api build-ui

build-api:
	cd api && CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o ../$(DIST)/proxmaid ./cmd/proxmaid/

build-ui:
	cd ui && npm run build && cp -r out ../$(DIST)/ui

test:
	cd api && go test ./... -v -count=1

test-short:
	cd api && go test ./... -count=1

lint:
	cd api && go vet ./...

clean:
	rm -rf $(DIST)

# Build .deb package
package: build
	mkdir -p $(DEB_ROOT)/usr/bin
	mkdir -p $(DEB_ROOT)/usr/share/proxmaid/ui
	mkdir -p $(DEB_ROOT)/lib/systemd/system
	mkdir -p $(DEB_ROOT)/DEBIAN
	cp $(DIST)/proxmaid $(DEB_ROOT)/usr/bin/proxmaid
	[ -d $(DIST)/ui ] && cp -r $(DIST)/ui/* $(DEB_ROOT)/usr/share/proxmaid/ui/ || true
	cp packaging/proxmaid.service $(DEB_ROOT)/lib/systemd/system/
	cp packaging/DEBIAN/control $(DEB_ROOT)/DEBIAN/
	cp packaging/DEBIAN/postinst $(DEB_ROOT)/DEBIAN/
	cp packaging/DEBIAN/prerm $(DEB_ROOT)/DEBIAN/
	sed -i "s/Version: .*/Version: $(VERSION)/" $(DEB_ROOT)/DEBIAN/control
	chmod 755 $(DEB_ROOT)/DEBIAN/postinst $(DEB_ROOT)/DEBIAN/prerm
	dpkg-deb --build $(DEB_ROOT) $(DIST)/proxmaid_$(VERSION)_amd64.deb

install:
	install -m 755 $(DIST)/proxmaid /usr/bin/proxmaid
	install -m 644 packaging/proxmaid.service /lib/systemd/system/proxmaid.service
	mkdir -p /etc/proxmaid
	systemctl daemon-reload
	systemctl enable proxmaid
