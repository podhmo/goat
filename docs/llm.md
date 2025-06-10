# About go/packages
Do not use `go/packages` because it makes imports eager.
This can lead to issues with resolving types and packages, especially in complex scenarios or when full type information across all dependencies is not strictly needed for the task at hand.
Prefer using `go/parser` for AST-level analysis and `golang.org/x/tools/go/types` for type checking when more control over the loading process is required, or use a more targeted loading mechanism if available (like the `lazyload` package in this project).

# go/packagesについて
`go/packages`はimportがeagerになってしまうので使わないこと。
