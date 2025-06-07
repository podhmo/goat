module github.com/podhmo/goat

// tool golang.org/x/tools/cmd/goimports
go 1.23.0

toolchain go1.23.9

require (
	github.com/stretchr/testify v1.10.0
	golang.org/x/tools v0.33.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg
