SRC_FILES:=$(shell find . -type f -path '*.go')

.PHONY: clean slow test testv

remake: $(SRC_FILES)
	go build
	./remake -ready

clean:
	$(foreach name, $(wildcard remake),rm -r $(name);)

install: remake
	go install

test: $(SRC_FILES)
	go test ./...

testv: $(SRC_FILES)
	go test ./... -v


SLEEP=sleep 1

slow: slow1
	@echo "done slept"

slow1: slow2
	@$(SLEEP)
	touch slow1
slow2: slow3
	@$(SLEEP)
	touch slow2
slow3: slow4
	@$(SLEEP)
	touch slow3
slow4: slow5
	@$(SLEEP)
	touch slow4
slow5: slow6
	@$(SLEEP)
	touch slow5
slow6:
	@$(SLEEP)
	touch slow6
