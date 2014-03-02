.PHONY:  test
all:  clean test

clean:
	rm -rf test/output

test:
	test/test.sh
