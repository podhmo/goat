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
	if len(enumValuesInts) == 0 { return OptionCodeSnippets{} }

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

	defaultValue := false
	if val, ok := opt.DefaultValue.(bool); ok {
		defaultValue = val
	}

	cliName := opt.CliName
	logic := fmt.Sprintf("flag.BoolVar(&%s.%s, %q, %t, %s)\n",
		optionsVarName, opt.Name, cliName, defaultValue, formattedHelpText)
	return OptionCodeSnippets{Logic: logic}
}

func (h *BoolHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	_ = opt
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
	return OptionCodeSnippets{Logic: "// TODO: Refine Bool required check, esp. for those defaulting to true.\n"}
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
	if baseType == "int" { defaultFlagCliValue = "0" }
	if baseType == "bool" { defaultFlagCliValue = "false" }

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
		if f, ok := opt.DefaultValue.(float64); ok { defaultFlagCliValue = int(f) }
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
	if len(enumValuesInts) == 0 { return OptionCodeSnippets{} }

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
		if b, ok := opt.DefaultValue.(bool); ok { defaultFlagCliValue = b }
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

	logic := fmt.Sprintf(`flag.Func(%q, %s, func(s string) error {
	if s != "" {
		%s[%q] = true
		%s.%s = strings.Split(s, ",")
	}
	return nil
})
`, opt.CliName, formattedHelpText, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: "// TODO: Implement proper stringSliceFlag logic from main_generator.go\n" + logic}
}

func (h *StringSliceHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *StringSliceHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`if (len(%s.%s) == 0 || (%s.%s == nil && %s == "nil")) && !%s[%q] && !%s {
	slog.ErrorContext(%s, "Missing required option", "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
`, optionsVarName, opt.Name, optionsVarName, opt.Name, initialDefaultVarName, isFlagExplicitlySetMapName, opt.CliName, envWasSetVarName, ctxVarName, opt.CliName, opt.CliName, opt.EnvVar)
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
`, optionsVarName, opt.Name, valStr, "context.Background()", opt.CliName, valStr)
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
	logic := fmt.Sprintf(`
// Required check for TextUnmarshaler %s - this is a placeholder
if !%s[%q] && !%s {
	// Complex check needed here involving current value vs initialDefaultVarName
}
`, opt.Name, isFlagExplicitlySetMapName, opt.CliName, envWasSetVarName )
	return OptionCodeSnippets{Logic: "// TODO: Refine TextUnmarshaler required check\n"}
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
	slog.DebugContext(%s, "Default value for pointer TextUnmarshaler %s skipped as field is nil and type instantiation is complex here.")
} else {
	if err := %s.%s.UnmarshalText([]byte(%q)); err != nil {
		slog.WarnContext(%s, "Failed to unmarshal default value for *TextUnmarshaler option", "option", %q, "default", %q, "error", err)
	}
}
`, optionsVarName, opt.Name, "context.Background()", opt.CliName, optionsVarName, opt.Name, valStr, "context.Background()", opt.CliName, valStr)
            return OptionCodeSnippets{Logic: "// TODO: Address *TextUnmarshaler default init complexity\n" + logic}
        }
    }
    return OptionCodeSnippets{}
}

func (h *TextUnmarshalerPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	logic := fmt.Sprintf(`
if %s.%s == nil {
	slog.DebugContext(%s, "Env var for pointer TextUnmarshaler %s skipped for actual assignment if field is nil and type instantiation is complex here. Value was: %s", %s)
}
if %s.%s != nil {
	if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
		slog.WarnContext(%s, "Failed to unmarshal environment variable for *TextUnmarshaler option", "variable", %q, "value", %s, "error", err)
	}
} else {
	slog.WarnContext(%s, "Cannot process env var for nil *TextUnmarshaler option without instantiation", "variable", %q, "value", %s)
}
`, optionsVarName, opt.Name, ctxVarName, opt.CliName, envValVarName, opt.EnvVar,
optionsVarName, opt.Name, optionsVarName, opt.Name, envValVarName, ctxVarName, opt.EnvVar, envValVarName,
ctxVarName, opt.EnvVar, envValVarName)
    return OptionCodeSnippets{Logic: "// TODO: Address *TextUnmarshaler env var processing for nil fields\n" + logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	tempFlagVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Str"
	declarations := fmt.Sprintf("var %s string\n", tempFlagVar)
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), false)
	formattedHelpText := formatHelpText(helpDetail)
	defaultStrVal := ""
	if opt.DefaultValue != nil {
		if ds, ok := opt.DefaultValue.(string); ok {
			defaultStrVal = ds
		}
	}

	logic := fmt.Sprintf("flag.StringVar(&%s, %q, %s, %s)\n",
		tempFlagVar, opt.CliName, formatHelpText(defaultStrVal), formattedHelpText)

	return OptionCodeSnippets{Declarations: declarations, Logic: logic, PostProcessing: "// PostProcessing needed for " + tempFlagVar}
}

func (h *TextUnmarshalerPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	tempFlagVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Str"

	logic := fmt.Sprintf(`
if %s[%q] {
	if %s.%s == nil {
		slog.WarnContext(ctx, "Attempting to set nil pointer TextUnmarshaler from flag. Instantiation is complex.", "option", %q)
	}
	if %s.%s != nil {
		if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal flag value for *TextUnmarshaler option", "option", %q, "value", %s, "error", err)
		}
	} else {
		slog.ErrorContext(ctx, "Cannot apply flag to nil *TextUnmarshaler without instantiation", "option", %q, "value", %s)
	}
}
`, isFlagExplicitlySetMapName, opt.CliName, optionsVarName, opt.Name, opt.CliName,
optionsVarName, opt.Name, optionsVarName, opt.Name, tempFlagVar, opt.CliName, tempFlagVar,
opt.CliName, tempFlagVar)
	return OptionCodeSnippets{Logic: logic} // Simplified return
}

func (h *TextUnmarshalerPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	return (&StringPtrHandler{}).GenerateRequiredCheckCode(opt, optionsVarName, isFlagExplicitlySetMapName, initialDefaultVarName, envWasSetVarName, ctxVarName)
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
// For now, StringSliceHandler and TextUnmarshaler(Ptr)Handler have simplified flag logic.
