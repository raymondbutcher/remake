SRC_FILES:=$(wildcard *.go)

.PHONY: clean test testv

remake: $(SRC_FILES)
	go build
	./remake -ready

clean:
	$(foreach name, $(wildcard remake),rm -r $(name);)

test: $(SRC_FILES)
	go test

testv: $(SRC_FILES)
	go test -v
