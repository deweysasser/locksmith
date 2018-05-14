
all: connectionLibrary.go accountLibrary.go changeLibrary.go

accountLibrary.go: keyLibrary.go
	sed \
	-e 's/Key/Account/g' \
	-e 's/keyLibrary/accountLibrary/g' \
	-e 's/go:generate.*/GENERATED FILE/' \
	$< > $@

changeLibrary.go: keyLibrary.go
	test -f $@ && touch $@ || sed \
	-e 's/Key/Change/g' \
	-e 's/keyLibrary/changeLibrary/g' \
	-e 's/go:generate.*/GENERATED FILE/' \
	$< > $@

connectionLibrary.go: keyLibrary.go
	sed \
	-e 's/data.Key/connection.Connection/g' \
	-e 's/Key/Connection/g' \
	-e 's/"\(github.com.*data\)"$$/&\n\t"github.com\/deweysasser\/locksmith\/connection"/g'     \
	-e 's/keyLibrary/connectionLibrary/g' \
	-e 's/KeyLibrary/ConnectionLibrary/g' \
	-e 's/go:generate.*/GENERATED FILE/' \
	$< > $@

