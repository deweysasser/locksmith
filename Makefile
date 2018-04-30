OSES=darwin windows linux

THISPACKAGE=github.com/deweysasser/locksmith

all: build test

test:
	go test ./...

build:
	go build .

install: test
	go install $(THISPACKAGE)

install-all: test
	for os in ${OSES}; do GOOS=$$os go install $(THISPACKAGE); done

package: install-all dist 
	zip -j dist/windows_amd64.zip ${GOPATH}/bin/locksmith.exe
	zip -j dist/darwin_amd64.zip ${GOPATH}/bin/darwin_amd64/locksmith
	zip -j dist/linux_amd64.zip ${GOPATH}/bin/linux_amd64/locksmith

dist:
	mkdir -p $@

dist/*.zip:
	zip -j $@ $<

