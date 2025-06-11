
EXAMPLES_DIRS := examples/hello examples/enum examples/fullset examples/noinit

examples-check:
#	go run ./cmd/goat/ help-message examples/fullset/main.go
	go run ./cmd/goat/ help-message examples/hello/main.go
.PHONY: examples-check

examples-emit:
	$(foreach dir,$(EXAMPLES_DIRS), go generate ./$(dir)/... ;)
.PHONY: examples-emit

# format the code
# need: go install golang.org/x/tools/cmd/goimports@latest
format:
	go install golang.org/x/tools/cmd/goimports@v0.20.0
	$(HOME)/go/bin/goimports -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
.PHONY: format

test:
	go test ./...
.PHONY: test