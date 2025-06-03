module github.com/podhmo/goat

go 1.22

// tool golang.org/x/tools/cmd/goimports

replace example.com/myexternalpkg => ./internal/analyzer/testdata/src/myexternalpkg

replace example.com/anotherpkg => ./internal/analyzer/testdata/src/anotherpkg
