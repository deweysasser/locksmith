.PHONY:  test
all:  clean test

clean:
	rm -rf test/output test/mock-servers

test:
	test/test.sh
