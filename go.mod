module github.com/podhmo/goat

go 1.22

// toolchain go1.23.10 // Remove toolchain as it's for newer Go versions

// tool golang.org/x/tools/cmd/goimports

require golang.org/x/tools v0.20.0 // Downgrade for Go 1.22 compatibility

require golang.org/x/mod v0.17.0 // indirect; Adjusted based on x/tools v0.20.0 typical dependencies

// for test
replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg
