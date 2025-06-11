module github.com/podhmo/goat

go 1.24.0

tool golang.org/x/tools/cmd/goimports

// for test
replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg

require golang.org/x/tools v0.34.0

require (
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
)
