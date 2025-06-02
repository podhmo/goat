package config

// Config holds the configuration for the goat tool itself,
// typically derived from its command-line arguments.
type Config struct {
	RunFuncName            string // Name of the target 'run' function (e.g., "run")
	OptionsInitializerName string // Name of the options initializer function (e.g., "NewOptions")
	TargetFile             string // Path to the target Go file to be processed
	// MainFuncName string // TODO: Name of the main function if not "main"
	// OutputFile string // TODO: Path for the generated file, if not in-place
}
