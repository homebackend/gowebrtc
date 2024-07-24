# builds service executable
.PHONY: build
build:
	go build -x -v -o ./bin/gowebrtc pkg/main.go

clean:
	rm -rvf bin build

debian: build
	mkdir -p build/debian/gowebrtc/DEBIAN \
		build/debian/gowebrtc/usr/local/bin \
		build/debian/gowebrtc/etc/gowebrtc/ \
		build/debian/gowebrtc/lib/systemd/system \
		build/debian/gowebrtc/usr/share/gowebrtc
	cp -v config/config.yaml.sample build/debian/gowebrtc/etc/gowebrtc
	cp -v bin/gowebrtc build/debian/gowebrtc/usr/local/bin
	cp -v gowebrtc.service build/debian/gowebrtc/lib/systemd/system
	cp -av html build/debian/gowebrtc/usr/share/gowebrtc
	cp -v debian.control build/debian/gowebrtc/DEBIAN/control
	dpkg-deb --build build/debian/gowebrtc

debian-install: debian
	sudo dpkg -i build/debian/gowebrtc.deb

debian-uninstall:
	sudo dpkg -r gowebrtc

.PHONY: test
test: build
	@./bin/gowebrtc server
