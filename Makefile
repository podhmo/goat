examples-check:
#	go run ./cmd/goat/ help-message examples/fullset/main.go
	go run ./cmd/goat/ help-message examples/hello/main.go
.PHONY: examples-check

examples-emit:
	go run ./cmd/goat/ emit examples/hello/main.go
	go run ./cmd/goat/ emit --initializer NewOptions -run Run examples/enum/main.go
	go run ./cmd/goat/ emit --initializer NewOptions --run run examples/fullset/main.go
	
.PHONY: examples-emit

# format the code
format:
	go install golang.org/x/tools/cmd/goimports@latest
	$(HOME)/go/bin/goimports -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
.PHONY: format

test:
	go test ./...
.PHONY: test