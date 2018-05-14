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

release: .release .release/branch .release/merge .release/version .release/changelog  .release/update-version .release/build .release/tag 

.release:
	mkdir $@

.release/branch: .release/version
	git symbolic-ref --short HEAD >> $@
	git stash save "Stash for release $$(cat .release/version)"
	git checkout master
	git log -n 1 --pretty=format:"%H" > .release/previous-commit

.release/changelog: .release .release/version
	git log --max-parents=1 --pretty=format:"* %B" $$(git describe --tags --abbrev=0)..> $@.tmp
	vi $@.tmp
	mv $@.tmp $@

.release/version: LAST=$(shell git tag -l | awk -F /  '/release/{print $$2}' | tail -n -1)

.release/version: .release
	echo $(LAST) | awk -F . '{print $$1"."$$2+1}' > $@

.release/build: version.go build test package .release
	touch $@

.release/update-version: .release/version
	sed -i -e "/^const Version/d" version.go
	echo "const Version string = \"$$(cat .release/version)\"" >> version.go
	git commit -m "Bump to $$(cat .release/version)" -a
	touch $@

.release/tag: .release/version .release/changelog
	git tag -s -m "$$(cat .release/changelog)"  release/$$(cat .release/version)

.release/merge: .release/version .release/branch
	git merge --no-ff $$(cat .release/branch) -m "Merging for $$(cat .release/version)"

commit-release:
	git push 
	git push --tags
	git checkout $$(cat .release/branch)
	rm -rf .release

abort-release:
	-git tag -d $$(cat .release/version)
	git checkout $$(cat .release/branch)
	rm -rf .release
	git reset --hard $$(cat .release/previous-commit)
