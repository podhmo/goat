module github.com/podhmo/goat

// tool golang.org/x/tools/cmd/goimports
go 1.23.0

toolchain go1.23.9

require golang.org/x/tools v0.33.0

require (
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
)

replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg
