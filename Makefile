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

# Platforms to ship binaries for.  darwin/arm64 matters: every Mac since 2020
# is arm64, and an amd64-only darwin build is the wrong artifact for most of
# them.
PLATFORMS=darwin/amd64 darwin/arm64 windows/amd64 linux/amd64

# package builds straight to dist/ with an explicit -o rather than running
# `go install` and guessing where the binary landed.  The old recipe guessed
# wrong three ways: it read ${GOPATH} from the environment, which is normally
# unset, so every path collapsed to /bin/...; it looked for the windows binary
# in bin/ when a cross build puts it in bin/windows_amd64/; and it looked for
# the linux one in bin/linux_amd64/ when a *native* build has no such
# subdirectory.  An explicit -o depends on none of that.
package: test dist
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		if [ "$$os" = windows ]; then ext=".exe"; fi; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -o dist/$${os}_$${arch}/locksmith$$ext . || exit 1; \
		( cd dist/$${os}_$${arch} && zip -q -j ../$${os}_$${arch}.zip locksmith$$ext ) || exit 1; \
	done

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
	git checkout main
	git log -n 1 --pretty=format:"%H" > .release/previous-commit

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
	rm -rf .release

abort-release:
	-git tag -d $$(cat .release/version)
	git checkout $$(cat .release/branch)
	rm -rf .release
	git reset --hard $$(cat .release/previous-commit)
