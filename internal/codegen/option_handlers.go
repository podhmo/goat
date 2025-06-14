package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

// StringHandler handles code generation for string options.
type StringHandler struct{}

func (h *StringHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != "" {
		valStr, ok := opt.DefaultValue.(string)
		if !ok {
			valStr = fmt.Sprintf("%v", opt.DefaultValue)
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s\n", optionsVarName, opt.Name, formatHelpText(valStr))}
	}
	return OptionCodeSnippets{}
}

func (h *StringHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s\n", optionsVarName, opt.Name, envValVarName)}
}

func (h *StringHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultValStr := ""
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			defaultValStr = valStr
		} else {
			defaultValStr = fmt.Sprintf("%v", opt.DefaultValue)
		}
	}

	logic := fmt.Sprintf("flag.StringVar(&%s.%s, %q, %s, %s)\n",
		optionsVarName, opt.Name, opt.CliName, formatHelpText(defaultValStr), formattedHelpText)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *StringHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf("%s.%s == %s && !%s[%q] && !%s",
		optionsVarName, opt.Name, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{}
	}
	enumValuesVar := stringutils.ToCamelCase(opt.Name) + "EnumValues"
	var enumValuesFormatted []string
	for _, ev := range effectiveEnums {
		enumValuesFormatted = append(enumValuesFormatted, formatHelpText(ev))
	}

	declarations := fmt.Sprintf("var %s = []string{%s}\n", enumValuesVar, strings.Join(enumValuesFormatted, ", "))
	logic := fmt.Sprintf(`
found := false
for _, validVal := range %s {
	if %s.%s == validVal {
		found = true
		break
	}
}
if !found {
	slog.ErrorContext(%s, "Invalid value for option", "option", %q, "value", %s.%s, "allowed", %s)
	return fmt.Errorf("invalid value for --%s: got %%q, expected one of %%v", %s.%s, %s)
}
`, enumValuesVar, optionsVarName, opt.Name, ctxVarName, opt.CliName, optionsVarName, opt.Name, enumValuesVar, opt.CliName, optionsVarName, opt.Name, enumValuesVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

// IntHandler implementation...
type IntHandler struct{}

func (h *IntHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valInt, ok := opt.DefaultValue.(float64)
		if !ok {
			return OptionCodeSnippets{Logic: fmt.Sprintf("// Default value for %s (int) has unexpected type: %T\n", opt.Name, opt.DefaultValue)}
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %d\n", optionsVarName, opt.Name, int(valInt))}
	}
	return OptionCodeSnippets{}
}

func (h *IntHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	tempVar := stringutils.ToCamelCase(opt.Name) + "EnvVal"
	declarations := fmt.Sprintf("var %s int\n", tempVar)
	logic := fmt.Sprintf(`
if v, err := strconv.Atoi(%s); err == nil {
	%s = v
	%s.%s = %s
} else {
	slog.WarnContext(%s, "Invalid integer value for environment variable", "variable", %q, "value", %s, "error", err)
}
`, envValVarName, tempVar, optionsVarName, opt.Name, tempVar, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *IntHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultVal := 0
	if opt.DefaultValue != nil {
		if valNum, ok := opt.DefaultValue.(float64); ok {
			defaultVal = int(valNum)
		}
	}
	logic := fmt.Sprintf("flag.IntVar(&%s.%s, %q, %d, %s)\n",
		optionsVarName, opt.Name, opt.CliName, defaultVal, formattedHelpText)
	return OptionCodeSnippets{Logic: logic}
}

func (h *IntHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *IntHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf("%s.%s == %s && !%s[%q] && !%s",
		optionsVarName, opt.Name, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)
	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *IntHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{}
	}
	enumValuesVar := stringutils.ToCamelCase(opt.Name) + "EnumValues"
	var enumValuesInts []string
	for _, evStr := range effectiveEnums {
		if _, err := strconv.Atoi(evStr); err == nil {
			enumValuesInts = append(enumValuesInts, evStr)
		}
	}
	if len(enumValuesInts) == 0 {
		return OptionCodeSnippets{}
	}

	declarations := fmt.Sprintf("var %s = []int{%s}\n", enumValuesVar, strings.Join(enumValuesInts, ", "))
	logic := fmt.Sprintf(`
found := false
for _, validVal := range %s {
	if %s.%s == validVal {
		found = true
		break
	}
}
if !found {
	slog.ErrorContext(%s, "Invalid value for option", "option", %q, "value", %s.%s, "allowed", %s)
	return fmt.Errorf("invalid value for --%s: got %%d, expected one of %%v", %s.%s, %s)
}
`, enumValuesVar, optionsVarName, opt.Name, ctxVarName, opt.CliName, optionsVarName, opt.Name, enumValuesVar, opt.CliName, optionsVarName, opt.Name, enumValuesVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

// BoolHandler implementation...
type BoolHandler struct{}

func (h *BoolHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valBool, ok := opt.DefaultValue.(bool)
		if !ok {
			return OptionCodeSnippets{Logic: fmt.Sprintf("// Default value for %s (bool) has unexpected type: %T\n", opt.Name, opt.DefaultValue)}
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %t\n", optionsVarName, opt.Name, valBool)}
	}
	return OptionCodeSnippets{}
}

func (h *BoolHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`
if v, err := strconv.ParseBool(%s); err == nil {
	%s.%s = v
} else {
	slog.WarnContext(%s, "Invalid boolean value for environment variable", "variable", %q, "value", %s, "error", err)
}
`, envValVarName, optionsVarName, opt.Name, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), true)
	formattedHelpText := formatHelpText(helpDetail)
	kebabCaseName := opt.CliName // Assuming CliName is already kebab-case

	declarations := ""
	logic := ""

	effectiveDefault := false
	if dv, ok := opt.DefaultValue.(bool); ok {
		effectiveDefault = dv
	}

	// Handle the special case for required bools defaulting to true, needing a --no-XXX flag
	if opt.IsRequired && effectiveDefault {
		noFlagVarName := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "NoFlagPresent"
		declarations += fmt.Sprintf("var %s bool\n", noFlagVarName)
		logic += fmt.Sprintf("flag.BoolVar(&%s, \"no-%s\", false, %s)\n",
			noFlagVarName, kebabCaseName, formatHelpText("Set "+kebabCaseName+" to false, overriding default true"))
		// The actual options field will remain true unless this --no-XXX flag is set.
		// The GenerateFlagPostParseAssignmentCode will handle setting it to false.
		// Also register the normal flag, but make it behave such that if only --option is given, it's true.
		// If options.MyBool is true (default), and user says --my-bool=false, it becomes false.
		// If options.MyBool is true (default), and user says nothing, it's true.
		// If options.MyBool is true (default), and user says --no-my-bool, it becomes false.
		// This means the flag.BoolVar should point to the options field.
		logic += fmt.Sprintf("flag.BoolVar(&%s.%s, %q, %t, %s)\n",
			optionsVarName, opt.Name, kebabCaseName, effectiveDefault, formattedHelpText)

	} else {
		logic = fmt.Sprintf("flag.BoolVar(&%s.%s, %q, %t, %s)\n",
			optionsVarName, opt.Name, kebabCaseName, effectiveDefault, formattedHelpText)
	}

	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *BoolHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	effectiveDefault := false
	if dv, ok := opt.DefaultValue.(bool); ok {
		effectiveDefault = dv
	}

	if opt.IsRequired && effectiveDefault {
		noFlagVarName := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "NoFlagPresent"
		// isFlagExplicitlySetMapName here should refer to the "no-XXX" flag.
		// The map key for `no-XXX` would be `no-` + opt.CliName.
		noFlagCliName := "no-" + opt.CliName
		logic := fmt.Sprintf(`if %s[%q] { // If --no-XXX flag was explicitly set
	%s.%s = false
}
`, isFlagExplicitlySetMapName, noFlagCliName, optionsVarName, opt.Name)
		// If the main flag (--XXX) was set to false, e.g. --mybool=false, that takes precedence.
		// flag.BoolVar directly modifies optionsVarName.opt.Name.
		// If --no-XXX is present, AND --XXX=true is present, the last one parsed wins or flag pkg errors.
		// Standard library flag parsing: last flag wins. So if --no-foo then --foo, foo is true.
		// This post-parse assignment ensures --no-foo makes it false even if --foo was also present (if it defaulted to true).
		return OptionCodeSnippets{Logic: logic}
	}
	return OptionCodeSnippets{}
}

func (h *BoolHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf("%s.%s == %s && !%s[%q] && !%s",
		optionsVarName, opt.Name, initialDefaultVarName,
		isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required boolean option (must be explicitly set)", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing or not explicitly set required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

// StringPtrHandler implementation...
type StringPtrHandler struct{}

func (h *StringPtrHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			tempVar := stringutils.ToCamelCase(opt.Name) + "DefaultValLiteral"
			declarations := fmt.Sprintf("%s := %s\n", tempVar, formatHelpText(valStr))
			logic := fmt.Sprintf("%s.%s = &%s\n", optionsVarName, opt.Name, tempVar)
			return OptionCodeSnippets{Declarations: declarations, Logic: logic}
		}
	}
	return OptionCodeSnippets{}
}

func (h *StringPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf("{\n valCopy := %s\n %s.%s = &valCopy\n}\n", envValVarName, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	defaultValForFlagVar := fmt.Sprintf("default%sValForFlag", stringutils.ToTitle(opt.Name))
	baseType := strings.TrimPrefix(opt.TypeName, "*")

	declarations := fmt.Sprintf("%s := %s.%s == nil\n", isNilInitiallyVar, optionsVarName, opt.Name)
	declarations += fmt.Sprintf("var %s %s\n", tempValVar, baseType)
	declarations += fmt.Sprintf("var %s %s\n", defaultValForFlagVar, baseType)
	declarations += fmt.Sprintf("if !%s { %s = *%s.%s }\n", isNilInitiallyVar, defaultValForFlagVar, optionsVarName, opt.Name)

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultFlagCliValue := ""
	if baseType == "int" {
		defaultFlagCliValue = "0"
	}
	if baseType == "bool" {
		defaultFlagCliValue = "false"
	}

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	flagRegLogic += fmt.Sprintf("	flag.StringVar(&%s, %q, %s, %s)\n", tempValVar, opt.CliName, formatHelpText(defaultFlagCliValue), formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.StringVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, opt.CliName, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *StringPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"

	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name, tempValVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf(`(%s.%s == nil || *%s.%s == "")`, optionsVarName, opt.Name, optionsVarName, opt.Name)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing or empty required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing or empty required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringPtrHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{}
	}
	enumValuesVar := stringutils.ToCamelCase(opt.Name) + "EnumValues"
	var enumValuesFormatted []string
	for _, ev := range effectiveEnums {
		enumValuesFormatted = append(enumValuesFormatted, formatHelpText(ev))
	}

	declarations := fmt.Sprintf("var %s = []string{%s}\n", enumValuesVar, strings.Join(enumValuesFormatted, ", "))
	logic := fmt.Sprintf(`
if %s.%s != nil {
	found := false
	for _, validVal := range %s {
		if *%s.%s == validVal {
			found = true
			break
		}
	}
	if !found {
		slog.ErrorContext(%s, "Invalid value for option", "option", %q, "value", *%s.%s, "allowed", %s)
		return fmt.Errorf("invalid value for --%s: got %%q, expected one of %%v", *%s.%s, %s)
	}
}
`, optionsVarName, opt.Name, enumValuesVar, optionsVarName, opt.Name, ctxVarName, opt.CliName, optionsVarName, opt.Name, enumValuesVar, opt.CliName, optionsVarName, opt.Name, enumValuesVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

// IntPtrHandler implementation...
type IntPtrHandler struct{}

func (h *IntPtrHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valNum, ok := opt.DefaultValue.(float64)
		if ok {
			valInt := int(valNum)
			tempVar := stringutils.ToCamelCase(opt.Name) + "DefaultVal"
			declarations := fmt.Sprintf("%s := %d\n", tempVar, valInt)
			logic := fmt.Sprintf("%s.%s = &%s\n", optionsVarName, opt.Name, tempVar)
			return OptionCodeSnippets{Declarations: declarations, Logic: logic}
		}
	}
	return OptionCodeSnippets{}
}

func (h *IntPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`
if v, err := strconv.Atoi(%s); err == nil {
	valCopy := v
	%s.%s = &valCopy
} else {
	slog.WarnContext(%s, "Invalid integer value for environment variable", "variable", %q, "value", %s, "error", err)
}
`, envValVarName, optionsVarName, opt.Name, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *IntPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	defaultValForFlagVar := fmt.Sprintf("default%sValForFlag", stringutils.ToTitle(opt.Name))
	baseType := strings.TrimPrefix(opt.TypeName, "*")

	declarations := fmt.Sprintf("%s := %s.%s == nil\n", isNilInitiallyVar, optionsVarName, opt.Name)
	declarations += fmt.Sprintf("var %s %s\n", tempValVar, baseType)
	declarations += fmt.Sprintf("var %s %s\n", defaultValForFlagVar, baseType)
	declarations += fmt.Sprintf("if !%s { %s = *%s.%s }\n", isNilInitiallyVar, defaultValForFlagVar, optionsVarName, opt.Name)

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultFlagCliValue := 0
	if opt.DefaultValue != nil {
		if f, ok := opt.DefaultValue.(float64); ok {
			defaultFlagCliValue = int(f)
		}
	}

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	flagRegLogic += fmt.Sprintf("	flag.IntVar(&%s, %q, %d, %s)\n", tempValVar, opt.CliName, defaultFlagCliValue, formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.IntVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, opt.CliName, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *IntPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name, tempValVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *IntPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf(`%s.%s == nil`, optionsVarName, opt.Name)
	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *IntPtrHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{}
	}
	enumValuesVar := stringutils.ToCamelCase(opt.Name) + "EnumValues"
	var enumValuesInts []string
	for _, evStr := range effectiveEnums {
		if _, err := strconv.Atoi(evStr); err == nil {
			enumValuesInts = append(enumValuesInts, evStr)
		}
	}
	if len(enumValuesInts) == 0 {
		return OptionCodeSnippets{}
	}

	declarations := fmt.Sprintf("var %s = []int{%s}\n", enumValuesVar, strings.Join(enumValuesInts, ", "))
	logic := fmt.Sprintf(`
if %s.%s != nil {
	found := false
	for _, validVal := range %s {
		if *%s.%s == validVal {
			found = true
			break
		}
	}
	if !found {
		slog.ErrorContext(%s, "Invalid value for option", "option", %q, "value", *%s.%s, "allowed", %s)
		return fmt.Errorf("invalid value for --%s: got %%d, expected one of %%v", *%s.%s, %s)
	}
}
`, optionsVarName, opt.Name, enumValuesVar, optionsVarName, opt.Name, ctxVarName, opt.CliName, optionsVarName, opt.Name, enumValuesVar, opt.CliName, optionsVarName, opt.Name, enumValuesVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

// BoolPtrHandler implementation...
type BoolPtrHandler struct{}

func (h *BoolPtrHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valBool, ok := opt.DefaultValue.(bool)
		if ok {
			tempVar := stringutils.ToCamelCase(opt.Name) + "DefaultVal"
			declarations := fmt.Sprintf("%s := %t\n", tempVar, valBool)
			logic := fmt.Sprintf("%s.%s = &%s\n", optionsVarName, opt.Name, tempVar)
			return OptionCodeSnippets{Declarations: declarations, Logic: logic}
		}
	}
	return OptionCodeSnippets{}
}

func (h *BoolPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`
if v, err := strconv.ParseBool(%s); err == nil {
    valCopy := v
    %s.%s = &valCopy
} else {
    slog.WarnContext(%s, "Invalid boolean value for environment variable", "variable", %q, "value", %s, "error", err)
}
`, envValVarName, optionsVarName, opt.Name, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	defaultValForFlagVar := fmt.Sprintf("default%sValForFlag", stringutils.ToTitle(opt.Name))
	baseType := strings.TrimPrefix(opt.TypeName, "*")

	declarations := fmt.Sprintf("%s := %s.%s == nil\n", isNilInitiallyVar, optionsVarName, opt.Name)
	declarations += fmt.Sprintf("var %s %s\n", tempValVar, baseType)
	declarations += fmt.Sprintf("var %s %s\n", defaultValForFlagVar, baseType)
	declarations += fmt.Sprintf("if !%s { %s = *%s.%s }\n", isNilInitiallyVar, defaultValForFlagVar, optionsVarName, opt.Name)

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), true)
	formattedHelpText := formatHelpText(helpDetail)

	defaultFlagCliValue := false
	if opt.DefaultValue != nil {
		if b, ok := opt.DefaultValue.(bool); ok {
			defaultFlagCliValue = b
		}
	}

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	flagRegLogic += fmt.Sprintf("	flag.BoolVar(&%s, %q, %t, %s)\n", tempValVar, opt.CliName, defaultFlagCliValue, formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.BoolVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, opt.CliName, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *BoolPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name, tempValVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf(`%s.%s == nil`, optionsVarName, opt.Name)
	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolPtrHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

// StringSliceHandler implementation...
type StringSliceHandler struct{}

func (h *StringSliceHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if strVal, ok := opt.DefaultValue.(string); ok && strVal != "" {
		parts := strings.Split(strVal, ",")
		var quotedParts []string
		for _, p := range parts {
			quotedParts = append(quotedParts, fmt.Sprintf("%q", strings.TrimSpace(p)))
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = []string{%s}\n", optionsVarName, opt.Name, strings.Join(quotedParts, ", "))}
	}
	return OptionCodeSnippets{}
}

func (h *StringSliceHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`if %s != "" {
	%s.%s = strings.Split(%s, ",")
}
`, envValVarName, optionsVarName, opt.Name, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringSliceHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	// Default value for string slice flag description.
	// If opt.DefaultValue is "a,b,c", it will be displayed as such.
	// The flag parsing logic will split it.
	defaultValStr := ""
	if opt.DefaultValue != nil {
		if dv, ok := opt.DefaultValue.(string); ok {
			defaultValStr = dv
		} else {
			// This case should ideally not happen if metadata is well-formed.
			// Fallback to string representation.
			defaultValStr = fmt.Sprintf("%v", opt.DefaultValue)
		}
	}

	// The flag package doesn't have a direct StringSliceVar that also gives us default value string representation easily.
	// We use flag.Func to handle parsing and to mark the flag as set.
	// The options.<field> is usually pre-populated with default values (if any)
	// or will be an empty slice. The flag.Func will override it if the flag is provided.

	logic := fmt.Sprintf(`flag.Func(%q, %s, func(s string) error {
	// No need to check if s is empty string, flag package calls this func only if flag is present.
	// If flag is present with an empty value (e.g. --my-slice=""), s will be ""
	// and we should treat it as explicitly setting the slice to empty or a slice with one empty string.
	// strings.Split("", ",") -> []string{""}
	// strings.Split("a,,b", ",") -> []string{"a", "", "b"}
	%s[%q] = true // Mark as explicitly set
	%s.%s = strings.Split(s, ",")
	return nil
})
`, opt.CliName, formattedHelpText, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name)

	// Note: The 'defaultValStr' is used by constructFlagHelpDetail to show the default in help.
	// The actual initial value of optionsVarName.opt.Name should be set by GenerateDefaultValueInitializationCode.
	// flag.Func does not take a "default value" argument in the same way flag.StringVar does for its value storage.
	// The third argument to flag.Func is the help text.
	// The initial value of the slice in the options struct serves as the "default" if the flag isn't used.

	return OptionCodeSnippets{Logic: logic}
}

func (h *StringSliceHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *StringSliceHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}

	// For a slice, "empty" typically means len == 0.
	// initialDefaultVarName is not directly comparable for slices like for simple types.
	// The condition focuses on whether the slice is empty AND was not explicitly set by flag or env var.
	condition := fmt.Sprintf("len(%s.%s) == 0 && !%s[%q] && !%s",
		optionsVarName, opt.Name, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *StringSliceHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

// TextUnmarshalerHandler handles code generation for types implementing encoding.TextUnmarshaler.
type TextUnmarshalerHandler struct{}

func (h *TextUnmarshalerHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			logic := fmt.Sprintf(`
if err := %s.%s.UnmarshalText([]byte(%q)); err != nil {
	slog.WarnContext(%s, "Failed to unmarshal default value for TextUnmarshaler option", "option", %q, "default", %q, "error", err)
}
`, optionsVarName, opt.Name, valStr, ctxVarName, opt.CliName, valStr) // Use ctxVarName
			return OptionCodeSnippets{Logic: logic}
		}
	}
	return OptionCodeSnippets{}
}

func (h *TextUnmarshalerHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`
if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
	slog.WarnContext(%s, "Failed to unmarshal environment variable for TextUnmarshaler option", "variable", %q, "value", %s, "error", err)
}
`, optionsVarName, opt.Name, envValVarName, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	logic := fmt.Sprintf(`
flag.Func(%q, %s, func(s string) error {
	%s[%q] = true
	return %s.%s.UnmarshalText([]byte(s))
})
`, opt.CliName, formattedHelpText, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: logic} // Simplified return
}

func (h *TextUnmarshalerHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *TextUnmarshalerHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}

	// initialDefaultVarName holds the string representation of the default value.
	// We need to marshal the current value back to text to compare.
	currentValTextVar := stringutils.ToCamelCase(opt.Name) + "CurrentTextVal"
	declarations := fmt.Sprintf("var %s []byte\n", currentValTextVar)
	logic := fmt.Sprintf(`
%s, err := %s.%s.MarshalText()
if err != nil {
	slog.WarnContext(%s, "Failed to marshal current value for TextUnmarshaler option for required check", "option", %q, "error", err)
	// If marshaling fails, we can't reliably compare. Assume it's different from default or proceed with caution.
	// Depending on strictness, one might choose to error here or allow proceeding.
	// For now, let's assume if it fails to marshal, it's problematic for a required check.
	return fmt.Errorf("failed to marshal current value for option %s for required check: %%w", err)
}
`, currentValTextVar, optionsVarName, opt.Name, ctxVarName, opt.CliName, opt.CliName)

	// Condition: current value (as text) is same as initial default text, AND flag not set, AND env var not set.
	condition := fmt.Sprintf("string(%s) == %s && !%s[%q] && !%s",
		currentValTextVar, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic += fmt.Sprintf(`
if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)

	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *TextUnmarshalerHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

// TextUnmarshalerPtrHandler handles code generation for *MyType where MyType implements encoding.TextUnmarshaler.
type TextUnmarshalerPtrHandler struct{}

func (h *TextUnmarshalerPtrHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			logic := fmt.Sprintf(`
if %s.%s == nil {
	%s.%s = new(%s) // Initialize if nil
}
if err := %s.%s.UnmarshalText([]byte(%q)); err != nil {
	slog.WarnContext(%s, "Failed to unmarshal default value for *TextUnmarshaler option", "option", %q, "default", %q, "error", err)
}
`, optionsVarName, opt.Name, optionsVarName, opt.Name, strings.TrimPrefix(opt.TypeName, "*"), optionsVarName, opt.Name, valStr, ctxVarName, opt.CliName, valStr)
			return OptionCodeSnippets{Logic: logic}
		}
	}
	return OptionCodeSnippets{}
}

func (h *TextUnmarshalerPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	actualType := strings.TrimPrefix(opt.TypeName, "*")
	logic := fmt.Sprintf(`
if %s.%s == nil {
	%s.%s = new(%s) // Initialize if nil
}
if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
	slog.WarnContext(%s, "Failed to unmarshal environment variable for *TextUnmarshaler option", "variable", %q, "value", %s, "error", err)
}
`, optionsVarName, opt.Name, optionsVarName, opt.Name, actualType,
		optionsVarName, opt.Name, envValVarName, ctxVarName, opt.EnvVar, envValVarName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	tempFlagVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Str"
	declarations := fmt.Sprintf("var %s string\n", tempFlagVar)

	// Use constructFlagHelpDetail for consistent help text formatting
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultStrVal := "" // Default for the temporary string flag variable is empty
	if opt.DefaultValue != nil {
		if ds, ok := opt.DefaultValue.(string); ok {
			// This default is for the help text via constructFlagHelpDetail.
			// The actual default for the flag variable itself should be empty,
			// so we only set it if the flag is NOT provided.
			// However, flag.StringVar needs a default value for its string var.
			// If options.Field is already non-nil and has a value, that value isn't directly usable here
			// unless we marshal it. For simplicity, flag string default is empty, matching if user provides `--opt=""`.
			// The actual options.Field default is handled by GenerateDefaultValueInitializationCode.
			// For the purpose of flag parsing, if the user has a default value like "defval" for the *MyType,
			// and they DON'T provide the flag, options.MyField should retain its "defval" (after UnmarshalText).
			// If they DO provide the flag e.g. --my-flag "newval", then "newval" is used.
			// If they provide --my-flag "" then "" is used.
			// So, for the flag.StringVar, the default for its temporary string variable should be ""
			// unless we want to reflect the field's initial default in the flag's empty case, which is complex.
			// Let's keep defaultStrVal for the flag as empty string for now.
			// The help text will show the intended default via `constructFlagHelpDetail`.
		}
	}

	logic := fmt.Sprintf("flag.StringVar(&%s, %q, %s, %s)\n",
		tempFlagVar, opt.CliName, formatHelpText(defaultStrVal), formattedHelpText)

	// PostProcessing is handled by GenerateFlagPostParseAssignmentCode, so no specific comment needed here.
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	tempFlagVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Str"
	actualType := strings.TrimPrefix(opt.TypeName, "*")

	logic := fmt.Sprintf(`
if %s[%q] { // Check if the flag was explicitly set
	if %s.%s == nil {
		%s.%s = new(%s) // Initialize if nil
	}
	if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
		slog.ErrorContext(%s, "Failed to unmarshal flag value for *TextUnmarshaler option", "option", %q, "value", %s, "error", err)
		// Potentially return an error here if strict parsing is required:
		// return fmt.Errorf("failed to unmarshal flag value for --%s ('%%s'): %%w", %s, err)
	}
}
`, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name, optionsVarName, opt.Name, actualType,
		optionsVarName, opt.Name, tempFlagVar, ctxVarName, opt.CliName, tempFlagVar,
		// For errorf: opt.CliName, tempFlagVar
	)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	// For a *TextUnmarshaler, "required" means the pointer must not be nil
	// after considering defaults, env vars, and flags.
	// The StringPtrHandler's check for `options.MyField == nil` is appropriate.
	// Whether the unmarshaled value itself is "empty" or "default" is specific to the type's implementation
	// and generally not checked at this generic level, unlike non-pointer TextUnmarshaler.
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}

	// Condition: field is nil AND it wasn't set by a flag AND it wasn't set by an env var.
	// Note: initialDefaultVarName is for the string representation of the default, not directly useful for nil check here.
	// Default initialization, env processing, and flag processing should have already attempted to initialize the pointer.
	// So, we just check if it's still nil.
	condition := fmt.Sprintf("%s.%s == nil && !%s[%q] && !%s",
		optionsVarName, opt.Name, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option (must be non-nil)", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option (must be non-nil): --%s / %s")
}
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

// UnsupportedTypeHandler handles types for which specific code generation is not yet implemented.
type UnsupportedTypeHandler struct{}

func (h *UnsupportedTypeHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("// Default value for unsupported type %s (%s) not handled.\n", opt.Name, opt.TypeName)}
}
func (h *UnsupportedTypeHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("// Environment variable for unsupported type %s (%s) not handled.\n", opt.Name, opt.TypeName)}
}
func (h *UnsupportedTypeHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("// Flag registration for unsupported type %s (%s) not handled.\n", opt.Name, opt.TypeName)}
}
func (h *UnsupportedTypeHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}
func (h *UnsupportedTypeHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("// Required check for unsupported type %s (%s) not handled.\n", opt.Name, opt.TypeName)}
}
func (h *UnsupportedTypeHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("// Enum validation for unsupported type %s (%s) not handled.\n", opt.Name, opt.TypeName)}
}

// constructFlagHelpDetail builds the detailed help string part that includes default values and enum choices.
// The actual formatting for Go literal (e.g. backticks) is done by main_generator.formatHelpText.
func constructFlagHelpDetail(baseHelpText string, defaultValue any, enumValues []string, isBool bool) string {
	if baseHelpText == "" {
		baseHelpText = "N/A"
	}
	parts := []string{baseHelpText}

	if defaultValue != nil {
		if b, ok := defaultValue.(bool); ok {
			if b {
				parts = append(parts, fmt.Sprintf("(default: %v)", defaultValue))
			}
		} else if strDefault, ok := defaultValue.(string); ok && strDefault == "" {
			// No default for empty string
		} else {
			parts = append(parts, fmt.Sprintf("(default: %v)", defaultValue))
		}
	}

	if len(enumValues) > 0 {
		parts = append(parts, fmt.Sprintf("(allowed: %s)", strings.Join(enumValues, ", ")))
	}
	return strings.Join(parts, " ")
}

// TODO: Define and implement stringSliceFlag and textUnmarshalerFlag types if they are to be used
// as they were in the original main_generator.go for proper slice/TextUnmarshaler flag handling.
// StringSliceHandler uses flag.Func. TextUnmarshaler(Ptr)Handler logic has been expanded.
