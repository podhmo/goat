module github.com/podhmo/goat

go 1.23.0

toolchain go1.23.10

// for test
replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg

require (
	golang.org/x/mod v0.25.0
	golang.org/x/tools v0.30.0
)

require golang.org/x/sync v0.11.0 // indirect
