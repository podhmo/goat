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
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if !ok {
			valStr = fmt.Sprintf("%v", opt.DefaultValue) // Coerce if not string (e.g. int to "123")
		}
		if valStr == "" { // If default is explicitly empty string, treat as no-op (rely on Go zero value)
			return OptionCodeSnippets{}
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s\n", optionsVarName, opt.Name, formatHelpText(valStr))}
	}
	return OptionCodeSnippets{} // If opt.DefaultValue was nil
}

func (h *StringHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s\n", optionsVarName, opt.Name, envValVarName)}
}

func (h *StringHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultValForFlag := ""
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			defaultValForFlag = valStr
		} else {
			defaultValForFlag = fmt.Sprintf("%v", opt.DefaultValue)
		}
	}

	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf("flag.StringVar(&%s.%s, %q, %s, %s)\n",
		optionsVarName, opt.Name, cliNameForFlag, formatHelpText(defaultValForFlag), formattedHelpText)
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
return nil
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

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
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
return nil // Added this line
`, enumValuesVar, optionsVarName, opt.Name, ctxVarName, cliNameForLog, optionsVarName, opt.Name, enumValuesVar, cliNameForLog, optionsVarName, opt.Name, enumValuesVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

// IntHandler implementation...
type IntHandler struct{}

func (h *IntHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		var val int
		switch v := opt.DefaultValue.(type) {
		case float64:
			val = int(v)
		case int:
			val = v
		case string:
			parsedVal, err := strconv.Atoi(v)
			if err == nil {
				val = parsedVal
			} else {
				return OptionCodeSnippets{Logic: fmt.Sprintf("// Default value for %s (int) could not be parsed from string '%s': %v\n", opt.Name, v, err)}
			}
		default:
			return OptionCodeSnippets{Logic: fmt.Sprintf("// Default value for %s (int) has unexpected type: %T\n", opt.Name, opt.DefaultValue)}
		}
		return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %d\n", optionsVarName, opt.Name, val)}
	}
	return OptionCodeSnippets{}
}

func (h *IntHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	tempVar := stringutils.ToCamelCase(opt.Name) + "EnvValConverted"
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
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultValForFlag := 0
	if opt.DefaultValue != nil {
		switch v := opt.DefaultValue.(type) {
		case float64:
			defaultValForFlag = int(v)
		case int:
			defaultValForFlag = v
		case string:
			parsedVal, err := strconv.Atoi(v)
			if err == nil {
				defaultValForFlag = parsedVal
			}
		}
	}
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf("flag.IntVar(&%s.%s, %q, %d, %s)\n",
		optionsVarName, opt.Name, cliNameForFlag, defaultValForFlag, formattedHelpText)
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
return nil
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

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
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
return nil // Added this line
`, enumValuesVar, optionsVarName, opt.Name, ctxVarName, cliNameForLog, optionsVarName, opt.Name, enumValuesVar, cliNameForLog, optionsVarName, opt.Name, enumValuesVar)
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
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, true)
	formattedHelpText := formatHelpText(helpDetail)
	kebabCaseName := opt.CliName
	if kebabCaseName == "" {
		kebabCaseName = stringutils.ToKebabCase(opt.Name)
	}

	declarations := ""
	logic := ""

	defaultValForFlag := false
	if opt.DefaultValue != nil {
		if dv, ok := opt.DefaultValue.(bool); ok {
			defaultValForFlag = dv
		}
	}

	if opt.IsRequired && defaultValForFlag {
		noFlagVarName := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "NoFlagPresent"
		declarations += fmt.Sprintf("var %s bool\n", noFlagVarName)
		logic += fmt.Sprintf("flag.BoolVar(&%s, \"no-%s\", false, %s)\n",
			noFlagVarName, kebabCaseName, formatHelpText("Set "+kebabCaseName+" to false, overriding default true"))
		logic += fmt.Sprintf("flag.BoolVar(&%s.%s, %q, %t, %s)\n",
			optionsVarName, opt.Name, kebabCaseName, defaultValForFlag, formattedHelpText)

	} else {
		logic = fmt.Sprintf("flag.BoolVar(&%s.%s, %q, %t, %s)\n",
			optionsVarName, opt.Name, kebabCaseName, defaultValForFlag, formattedHelpText)
	}

	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *BoolHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	defaultValForFlag := false
	if opt.DefaultValue != nil {
		if dv, ok := opt.DefaultValue.(bool); ok {
			defaultValForFlag = dv
		}
	}

	kebabCaseName := opt.CliName
	if kebabCaseName == "" {
		kebabCaseName = stringutils.ToKebabCase(opt.Name)
	}

	if opt.IsRequired && defaultValForFlag {
		noFlagCliName := "no-" + kebabCaseName
		logic := fmt.Sprintf(`if %s[%q] {
	%s.%s = false
}
`, isFlagExplicitlySetMapName, noFlagCliName, optionsVarName, opt.Name)
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
return nil
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

	defaultFlagCliValueStr := ""
	if opt.DefaultValue != nil {
		if strVal, ok := opt.DefaultValue.(string); ok {
			defaultFlagCliValueStr = strVal
		}
	}

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	flagRegLogic += fmt.Sprintf("	flag.StringVar(&%s, %q, %s, %s)\n", tempValVar, cliNameForFlag, formatHelpText(defaultFlagCliValueStr), formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.StringVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, cliNameForFlag, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *StringPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name, tempValVar)
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

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
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
return nil // Added this line
`, optionsVarName, opt.Name, enumValuesVar, optionsVarName, opt.Name, ctxVarName, cliNameForLog, optionsVarName, opt.Name, enumValuesVar, cliNameForLog, optionsVarName, opt.Name, enumValuesVar)
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

	defaultFlagCliValueInt := 0
	if opt.DefaultValue != nil {
		switch v := opt.DefaultValue.(type) {
		case float64:
			defaultFlagCliValueInt = int(v)
		case int:
			defaultFlagCliValueInt = v
		}
	}

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	flagRegLogic += fmt.Sprintf("	flag.IntVar(&%s, %q, %d, %s)\n", tempValVar, cliNameForFlag, defaultFlagCliValueInt, formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.IntVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, cliNameForFlag, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *IntPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name, tempValVar)
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
return nil
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

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
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
return nil // Added this line
`, optionsVarName, opt.Name, enumValuesVar, optionsVarName, opt.Name, ctxVarName, cliNameForLog, optionsVarName, opt.Name, enumValuesVar, cliNameForLog, optionsVarName, opt.Name, enumValuesVar)
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

	defaultFlagCliValueBool := false
	if opt.DefaultValue != nil {
		if b, ok := opt.DefaultValue.(bool); ok {
			defaultFlagCliValueBool = b
		}
	}

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, true)
	formattedHelpText := formatHelpText(helpDetail)

	flagRegLogic := fmt.Sprintf("if %s {\n", isNilInitiallyVar)
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	flagRegLogic += fmt.Sprintf("	flag.BoolVar(&%s, %q, %t, %s)\n", tempValVar, cliNameForFlag, defaultFlagCliValueBool, formattedHelpText)
	flagRegLogic += "} else {\n"
	flagRegLogic += fmt.Sprintf("	flag.BoolVar(%s.%s, %q, %s, %s)\n", optionsVarName, opt.Name, cliNameForFlag, defaultValForFlagVar, formattedHelpText)
	flagRegLogic += "}\n"

	return OptionCodeSnippets{Declarations: declarations, Logic: flagRegLogic}
}

func (h *BoolPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	isNilInitiallyVar := fmt.Sprintf("is%sNilInitially", stringutils.ToTitle(opt.Name))
	tempValVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf(`if %s && %s[%q] {
	%s.%s = &%s
}
`, isNilInitiallyVar, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name, tempValVar)
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
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf(`flag.Func(%q, %s, func(s string) error {
	%s[%q] = true
	%s.%s = strings.Split(s, ",")
	return nil
})
`, cliNameForFlag, formattedHelpText, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name)

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

// EnumHandler handles code generation for enum options.
type EnumHandler struct{}

func (h *EnumHandler) getActualTypeName(opt *metadata.OptionMetadata) string {
	if opt.TypePackage != "" {
		return fmt.Sprintf("%s.%s", opt.TypePackage, opt.TypeName)
	}
	return opt.TypeName
}

func (h *EnumHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue == nil {
		return OptionCodeSnippets{}
	}

	actualTypeName := h.getActualTypeName(opt)
	defaultValueStr := fmt.Sprintf("%v", opt.DefaultValue) // Works for int, string enums

	// For string-based enums, the value needs to be quoted in the generated code.
	// We assume other enum base types (like int) don't need special formatting here beyond %v.
	// The metadata.OptionMetadata.Type field could be enhanced to specify "string" vs "int" base for enums.
	// For now, we check if DefaultValue is a string.
	if _, ok := opt.DefaultValue.(string); ok {
		defaultValueStr = formatHelpText(defaultValueStr) // formatHelpText also quotes strings
	}

	return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s(%s)\n", optionsVarName, opt.Name, actualTypeName, defaultValueStr)}
}

func (h *EnumHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	actualTypeName := h.getActualTypeName(opt)
	// Assuming env var is a string, which is typical. The enum type constructor should handle it.
	return OptionCodeSnippets{Logic: fmt.Sprintf("%s.%s = %s(%s)\n", optionsVarName, opt.Name, actualTypeName, envValVarName)}
}

func (h *EnumHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	actualTypeName := h.getActualTypeName(opt)
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}

	// Construct help text including allowed values
	allowedValues := GetEffectiveEnumValues(opt)
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, allowedValues, opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	logic := fmt.Sprintf(`flag.Func(%q, %s, func(s string) error {
	%s[%q] = true
	%s.%s = %s(s)
	return nil
})
`, cliNameForFlag, formattedHelpText, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name, actualTypeName)
	return OptionCodeSnippets{Logic: logic}
}

func (h *EnumHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{} // Non-pointer enums are directly assigned by flag.Func
}

func (h *EnumHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}

	// Compare the current value (as string) to the initial default value (as string)
	// This handles enums that might be int-based or string-based.
	condition := fmt.Sprintf("fmt.Sprintf(\"%%v\", %s.%s) == %s && !%s[%q] && !%s",
		optionsVarName, opt.Name, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
return nil
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *EnumHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{} // No validation if no enum values defined
	}

	// actualTypeName := h.getActualTypeName(opt) // Removed as it's not used in the string comparison logic below
	// enumValuesVar := stringutils.ToCamelCase(opt.Name) + "EnumValues" // Removed as errorEnumListVar is used

	var enumValuesFormatted []string
	// We need to format them as strings for the "allowed" list in the error message.
	// And for comparison, we cast the field to string.
	for _, ev := range effectiveEnums {
		// If the enum values themselves are strings, they need to be quoted when constructing the slice of valid string representations
		// For example, if MyEnum can be "foo" or "bar", the generated code for enumValuesVar might be:
		// var myEnumEnumValues = []string{"\"foo\"", "\"bar\""} if we directly use formatHelpText(ev)
		// However, for comparison like string(options.MyEnumField) == validVal, validVal should be "foo", not "\"foo\""
		// So, we store the raw string values for comparison, and a separately formatted list for the error message.
		enumValuesFormatted = append(enumValuesFormatted, fmt.Sprintf("%q", ev)) // For error message display
	}

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
	}

	// This list is for the error message, so values should be quoted if they are strings
	errorEnumListVar := stringutils.ToCamelCase(opt.Name) + "ErrorEnumList"
	declarations := fmt.Sprintf("var %s = []string{%s}\n", errorEnumListVar, strings.Join(enumValuesFormatted, ", "))

	// For the actual comparison, we'll iterate over the raw effectiveEnums
	// and cast the current option value to a string for comparison.

	validationLogic := fmt.Sprintf(`
found := false
currentValueStr := fmt.Sprintf("%%v", %s.%s)
for _, validValStr := range %#v { // Use raw enum values for comparison logic
	if currentValueStr == validValStr {
		found = true
		break
	}
}
if !found {
	slog.ErrorContext(%s, "Invalid value for enum option", "option", %q, "value", %s.%s, "allowed", %s)
	return fmt.Errorf("invalid value for --%s: got %%v, expected one of %%v", %s.%s, %s)
}
return nil
`, optionsVarName, opt.Name, effectiveEnums, ctxVarName, cliNameForLog, optionsVarName, opt.Name, errorEnumListVar, cliNameForLog, optionsVarName, opt.Name, errorEnumListVar)

	return OptionCodeSnippets{Declarations: declarations, Logic: validationLogic}
}

// EnumPtrHandler handles code generation for pointer to enum options.
type EnumPtrHandler struct{}

func (h *EnumPtrHandler) getActualTypeName(opt *metadata.OptionMetadata) string {
	// For a pointer type like *customtypes.MyEnum, TypeName is *customtypes.MyEnum.
	// We need the base type for casting: customtypes.MyEnum.
	baseTypeName := strings.TrimPrefix(opt.TypeName, "*")
	if opt.TypePackage != "" {
		// If TypePackage is present, it means TypeName might be just "MyEnum"
		// and TypePackage is "customtypes". Or TypeName could be "customtypes.MyEnum"
		// and TypePackage is "customtypes" (a bit redundant but possible).
		// A robust way is to ensure the package prefix isn't duplicated if already in baseTypeName.
		if strings.HasPrefix(baseTypeName, opt.TypePackage+".") {
			return baseTypeName // e.g. baseTypeName = "customtypes.MyEnum", opt.TypePackage = "customtypes"
		}
		return fmt.Sprintf("%s.%s", opt.TypePackage, baseTypeName)
	}
	return baseTypeName
}

func (h *EnumPtrHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue == nil {
		return OptionCodeSnippets{}
	}

	actualBaseTypeName := h.getActualTypeName(opt) // This gets "MyEnum" or "pkg.MyEnum"
	defaultValueStr := fmt.Sprintf("%v", opt.DefaultValue)

	if _, ok := opt.DefaultValue.(string); ok {
		defaultValueStr = formatHelpText(defaultValueStr) // Quote if string
	}

	tempVar := stringutils.ToCamelCase(opt.Name) + "DefaultVal"
	declarations := fmt.Sprintf("%s := %s(%s)\n", tempVar, actualBaseTypeName, defaultValueStr)
	logic := fmt.Sprintf("%s.%s = &%s\n", optionsVarName, opt.Name, tempVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *EnumPtrHandler) GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets {
	actualBaseTypeName := h.getActualTypeName(opt)
	logic := fmt.Sprintf("{\n valCopy := %s(%s)\n %s.%s = &valCopy\n}\n", actualBaseTypeName, envValVarName, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: logic}
}

func (h *EnumPtrHandler) GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	// actualBaseTypeName := h.getActualTypeName(opt) // Removed as not directly used in this method; tempFlagStrVar is always string.
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}

	// Temporary variable to hold the string from the flag
	tempFlagStrVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "FlagStr"
	declarations := fmt.Sprintf("var %s string\n", tempFlagStrVar)

	// Default value for the flag itself (always string for flag.StringVar)
	defaultFlagValueStr := ""
	if opt.DefaultValue != nil {
		// opt.DefaultValue is the actual enum value, convert to string for the flag's default display
		defaultFlagValueStr = fmt.Sprintf("%v", opt.DefaultValue)
	}

	allowedValues := GetEffectiveEnumValues(opt)
	helpDetail := constructFlagHelpDetail(opt.HelpText, defaultFlagValueStr, allowedValues, opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	// We use flag.StringVar to capture the input. Post-processing will convert and assign the pointer.
	logic := fmt.Sprintf("flag.StringVar(&%s, %q, %s, %s)\n",
		tempFlagStrVar, cliNameForFlag, formatHelpText(defaultFlagValueStr), formattedHelpText)

	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *EnumPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	actualBaseTypeName := h.getActualTypeName(opt)
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	tempFlagStrVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "FlagStr"

	logic := fmt.Sprintf(`
if %s[%q] { // If flag was explicitly set
	valCopy := %s(%s)
	%s.%s = &valCopy
}
`, isFlagExplicitlySetMapName, cliNameForFlag, actualBaseTypeName, tempFlagStrVar, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: logic}
}

func (h *EnumPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	// For pointers, required means it must not be nil.
	// The initialDefaultVarName is not directly comparable if it's a pointer type in the generated code.
	// We check if the flag was set, or env var was set, or if it's non-nil.
	condition := fmt.Sprintf("%s.%s == nil && !%s[%q] && !%s",
		optionsVarName, opt.Name, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option (must be non-nil): --%s / %s")
}
return nil
`, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)
	return OptionCodeSnippets{Logic: logic}
}

func (h *EnumPtrHandler) GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets {
	effectiveEnums := GetEffectiveEnumValues(opt)
	if len(effectiveEnums) == 0 {
		return OptionCodeSnippets{}
	}

	// actualBaseTypeName := h.getActualTypeName(opt)
	var enumValuesFormatted []string
	for _, ev := range effectiveEnums {
		enumValuesFormatted = append(enumValuesFormatted, fmt.Sprintf("%q", ev))
	}

	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = stringutils.ToKebabCase(opt.Name)
	}

	errorEnumListVar := stringutils.ToCamelCase(opt.Name) + "ErrorEnumList"
	declarations := fmt.Sprintf("var %s = []string{%s}\n", errorEnumListVar, strings.Join(enumValuesFormatted, ", "))

	validationLogic := fmt.Sprintf(`
if %s.%s != nil {
	found := false
	currentValueStr := fmt.Sprintf("%%v", *%s.%s)
	for _, validValStr := range %#v { // Use raw enum values
		if currentValueStr == validValStr {
			found = true
			break
		}
	}
	if !found {
		slog.ErrorContext(%s, "Invalid value for enum option", "option", %q, "value", *%s.%s, "allowed", %s)
		return fmt.Errorf("invalid value for --%s: got %%v, expected one of %%v", *%s.%s, %s)
	}
}
return nil
`, optionsVarName, opt.Name, optionsVarName, opt.Name, effectiveEnums, ctxVarName, cliNameForLog, optionsVarName, opt.Name, errorEnumListVar, cliNameForLog, optionsVarName, opt.Name, errorEnumListVar)
	return OptionCodeSnippets{Declarations: declarations, Logic: validationLogic}
}

// TextUnmarshalerHandler handles code generation for types implementing encoding.TextUnmarshaler.
type TextUnmarshalerHandler struct{}

func (h *TextUnmarshalerHandler) GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets {
	if opt.DefaultValue != nil {
		valStr, ok := opt.DefaultValue.(string)
		if ok {
			cliNameForLog := opt.CliName
			if cliNameForLog == "" {
				cliNameForLog = stringutils.ToKebabCase(opt.Name)
			}
			logic := fmt.Sprintf(`
if err := %s.%s.UnmarshalText([]byte(%q)); err != nil {
	slog.WarnContext(ctxVarName, "Failed to unmarshal default value for TextUnmarshaler option", "option", %q, "default", %q, "error", err)
}
`, optionsVarName, opt.Name, valStr, cliNameForLog, valStr)
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
	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf(`
flag.Func(%q, %s, func(s string) error {
	%s[%q] = true
	return %s.%s.UnmarshalText([]byte(s))
})
`, cliNameForFlag, formattedHelpText, isFlagExplicitlySetMapName, cliNameForFlag, optionsVarName, opt.Name)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	return OptionCodeSnippets{}
}

func (h *TextUnmarshalerHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	cliNameForLog := opt.CliName
	if cliNameForLog == "" {
		cliNameForLog = kebabCaseName
	}
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}

	currentValTextVar := stringutils.ToCamelCase(opt.Name) + "CurrentTextVal"
	declarations := fmt.Sprintf("var %s []byte\n", currentValTextVar)
	logic := fmt.Sprintf(`
%s, err := %s.%s.MarshalText()
if err != nil {
	slog.WarnContext(%s, "Failed to marshal current value for TextUnmarshaler option for required check", "option", %q, "error", err)
	return fmt.Errorf("failed to marshal current value for option %s for required check: %%w", err)
}
`, currentValTextVar, optionsVarName, opt.Name, ctxVarName, cliNameForLog, cliNameForLog)

	condition := fmt.Sprintf("string(%s) == %s && !%s[%q] && !%s",
		currentValTextVar, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic += fmt.Sprintf(`
if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
return nil // Added this line
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
			cliNameForLog := opt.CliName
			if cliNameForLog == "" {
				cliNameForLog = stringutils.ToKebabCase(opt.Name)
			}
			logic := fmt.Sprintf(`
if %s.%s == nil {
	%s.%s = new(%s) // Initialize if nil
}
if err := %s.%s.UnmarshalText([]byte(%q)); err != nil {
	slog.WarnContext(ctxVarName, "Failed to unmarshal default value for *TextUnmarshaler option", "option", %q, "default", %q, "error", err)
}
`, optionsVarName, opt.Name, optionsVarName, opt.Name, strings.TrimPrefix(opt.TypeName, "*"), optionsVarName, opt.Name, valStr, cliNameForLog, valStr)
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

	helpDetail := constructFlagHelpDetail(opt.HelpText, opt.DefaultValue, GetEffectiveEnumValues(opt), opt.EnvVar, false)
	formattedHelpText := formatHelpText(helpDetail)

	defaultValStr := ""
	if opt.DefaultValue != nil {
		if s, ok := opt.DefaultValue.(string); ok {
			defaultValStr = s
		}
	}

	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}
	logic := fmt.Sprintf("flag.StringVar(&%s, %q, %s, %s)\n",
		tempFlagVar, cliNameForFlag, formatHelpText(defaultValStr), formattedHelpText)

	return OptionCodeSnippets{Declarations: declarations, Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets {
	tempFlagVar := globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Str"
	actualType := strings.TrimPrefix(opt.TypeName, "*")
	cliNameForFlag := opt.CliName
	if cliNameForFlag == "" {
		cliNameForFlag = stringutils.ToKebabCase(opt.Name)
	}

	logic := fmt.Sprintf(`
if %s[%q] { // Check if the flag was explicitly set
	if %s.%s == nil {
		%s.%s = new(%s) // Initialize if nil
	}
	if err := %s.%s.UnmarshalText([]byte(%s)); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal flag value for *TextUnmarshaler option", "option", %q, "value", %s, "error", err)
	}
}
`,
		isFlagExplicitlySetMapName, cliNameForFlag, // For outer if
		optionsVarName, opt.Name, // For options.Field == nil
		optionsVarName, opt.Name, actualType, // For new(ActualType)
		optionsVarName, opt.Name, tempFlagVar, // For UnmarshalText
		cliNameForFlag, tempFlagVar, // For slog: %q for option, %s for value
	)
	return OptionCodeSnippets{Logic: logic}
}

func (h *TextUnmarshalerPtrHandler) GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets {
	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := ""
	if opt.EnvVar != "" {
		envVarLogIfPresent = fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	}
	condition := fmt.Sprintf("%s.%s == nil && !%s[%q] && !%s",
		optionsVarName, opt.Name, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	logic := fmt.Sprintf(`if %s {
	slog.ErrorContext(%s, "Missing required option (must be non-nil)", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option (must be non-nil): --%s / %s")
}
return nil // Added this line
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
func constructFlagHelpDetail(baseHelpText string, defaultValue any, enumValues []string, envVar string, isBool bool) string {
	if baseHelpText == "" {
		baseHelpText = "N/A"
	}
	parts := []string{baseHelpText}

	if defaultValue != nil {
		if !strings.Contains(baseHelpText, "(default:") {
			if strDefault, ok := defaultValue.(string); ok && strDefault == "" {
				// Do not append "(default: )" for empty string defaults
			} else {
				parts = append(parts, fmt.Sprintf("(default: %v)", defaultValue))
			}
		}
	} else if isBool { // defaultValue is nil
		if !strings.Contains(baseHelpText, "(default:") {
			parts = append(parts, "(default: false)")
		}
	}

	if len(enumValues) > 0 {
		parts = append(parts, fmt.Sprintf("(allowed: %s)", strings.Join(enumValues, ", ")))
	}
	if envVar != "" {
		parts = append(parts, fmt.Sprintf("(env: %s)", envVar))
	}
	return strings.Join(parts, " ")
}
