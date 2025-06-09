
examples-check:
#	go run ./cmd/goat/ help-message examples/fullset/main.go
	go run ./cmd/goat/ help-message examples/hello/main.go
.PHONY: examples-check

examples-emit:
	go run ./cmd/goat/ emit examples/hello/main.go
	go run ./cmd/goat/ emit -run FullsetRun -initializer NewFullsetOptions examples/fullset/main.go
.PHONY: examples-emit

# format the code
# need: go install golang.org/x/tools/cmd/goimports@latest
format:
	grep "^tool " go.mod | sed 's/^tool //' | xargs go install
	go tool goimports -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
.PHONY: format

test:
	go test ./...
.PHONY: test