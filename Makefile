
check:
#	go run ./cmd/goat/ examples/fullset/main.go
	go run ./cmd/goat/ examples/hello/main.go
.PHONY: check

# format the code
# need: go install golang.org/x/tools/cmd/goimports@latest
format:
	goimports -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
.PHONY: format