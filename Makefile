PACKAGE=locksmith
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

# while we're packaging, the definition of VERSION is the contents of the .release/version file if we have it, otherwise "dev"
package: VERSION=$(shell test -f .release/version && cat .release/version || echo "dev")

package: install-all dist 
	zip -j dist/$(PACKAGE)-$(VERSION)-windows_amd64.zip ${GOPATH}/bin/locksmith.exe
	zip -j dist/$(PACKAGE)-$(VERSION)-darwin_amd64.zip ${GOPATH}/bin/darwin_amd64/locksmith
	zip -j dist/$(PACKAGE)-$(VERSION)-linux_amd64.zip ${GOPATH}/bin/linux_amd64/locksmith

dist:
	mkdir -p $@

dist/*.zip:
	zip -j $@ $<

release: .release .release/branch .release/merge .release/version .release/changelog  .release/update-version .release/build .release/tag 

.release:
	mkdir $@

.release/branch: .release/version
	git symbolic-ref --short HEAD > $@
	git stash save "Stash for release $$(cat .release/version)"
	git checkout master
	git log -n 1 --pretty=format:"%H" > .release/previous-master-commit

.release/changelog: .release .release/version
	git log --max-parents=1 --pretty=format:"* %B" $$(git describe --tags --abbrev=0)..> $@.tmp
	vi $@.tmp
	mv $@.tmp $@

.release/version: LAST=$(shell git tag -l | awk -F /  '/release/{print $$2}' | sort -V | tail -n -1)

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
#       if there was a stash, pop it
	git stash list | head -n 1 | grep "Stash for release $$(cat .release/version)" && git stash pop
	rm -rf .release

abort-release:
	-git tag -d release/$$(cat .release/version)
#       still on release branch (master)
	git reset --hard $$(cat .release/previous-master-commit)
#       move to working branch
	git checkout $$(cat .release/branch)
#       if there was a stash, pop it
	git stash list | head -n 1 | grep "Stash for release $$(cat .release/version)" && git stash pop || true
#       clear release state
	rm -rf .release
