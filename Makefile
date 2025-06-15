GOAT ?= .gobin/goat

examples-check:
	go build -o .gobin/goat ./cmd/goat
#	$(GOAT) help-message examples/fullset/main.go
	$(GOAT) help-message examples/hello/main.go
	
.PHONY: examples-check

examples-emit:
	go build -o .gobin/goat ./cmd/goat
	$(GOAT) emit examples/hello/main.go
	$(GOAT) emit --initializer NewOptions -run run examples/enum/main.go
	$(GOAT) emit --initializer NewOptions --run run examples/fullset/main.go

.PHONY: examples-emit

examples-scan:
	go build -o .gobin/goat ./cmd/goat
	$(GOAT) scan examples/hello/main.go > examples/hello/scan-result.json
	$(GOAT) scan --initializer NewOptions -run run examples/enum/main.go > examples/enum/scan-result.json
	$(GOAT) scan --initializer NewOptions --run run examples/fullset/main.go > examples/fullset/scan-result.json

.PHONY: examples-scan

# format the code
format:
	go install golang.org/x/tools/cmd/goimports@v0.20.0
	$(HOME)/go/bin/goimports -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
.PHONY: format

test:
	go test ./...
.PHONY: test