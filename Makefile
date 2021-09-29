SRC_FILES:=$(shell find . -type f -path '*.go')

.PHONY: all
all: fmt test remake

remake: $(SRC_FILES)
	go build
	./remake -ready

.PHONY: clean
clean:
	$(foreach name, $(wildcard remake slow*),rm -r $(name);)

.PHONY: fmt
fmt: $(SRC_FILES)
	go fmt ./...

.PHONY: test
test: $(SRC_FILES)
	go test ./...

.PHONY: testv
testv: $(SRC_FILES)
	go test ./... -v

.PHONY: slow
slow: slow1
	@echo "done slept"

SLEEP=sleep 1

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
