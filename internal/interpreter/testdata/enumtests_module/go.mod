module testdata/enumtests_module

go 1.18

// If your main test module uses replace directives for local paths,
// replicate them if necessary for the loader to correctly resolve imports
// like 'testcmdmodule/internal/goat'.
// For example, if 'testcmdmodule/internal/goat' is actually 'github.com/podhmo/goat'
// but aliased in the main go.mod for testing purposes:
// replace testcmdmodule/internal/goat => ../../../../ (adjust path to actual goat root from this go.mod)
// The key is that the loader, when running from the context of this go.mod,
// needs to be able to find the source code for 'testcmdmodule/internal/goat'.
// If the main project's go.mod already makes 'testcmdmodule/internal/goat' resolvable
// (e.g., it *is* the module path or replaced effectively), then this might not be needed.
// For this test, we assume 'testcmdmodule/internal/goat' is resolvable by the loader
// perhaps due to the main project's go.mod or test setup.
