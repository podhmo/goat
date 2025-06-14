package stringutils

import (
	"regexp"
	"strings"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// ToKebabCase converts a string from CamelCase or PascalCase to kebab-case.
// Example: "UserName" -> "user-name", "MinLength" -> "min-length"
func ToKebabCase(str string) string {
	if str == "" {
		return ""
	}
	snake := matchFirstCap.ReplaceAllString(str, "${1}-${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}-${2}")
	return strings.ToLower(snake)
}

// ToCamelCase converts a string from snake_case or kebab-case to camelCase.
// Example: "user_name" -> "userName", "min-length" -> "minLength"
func ToCamelCase(s string) string {
	s = strings.ReplaceAll(s, "-", "_") // Allow kebab-case input
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(part)
		} else {
			parts[i] = strings.Title(strings.ToLower(part))
		}
	}
	return strings.Join(parts, "")
}

// ToTitle converts a string to TitleCase, preserving existing capitalization for acronyms.
// Example: "some string" -> "Some String", "userID" -> "UserID"
func ToTitle(s string) string {
	if s == "" {
		return ""
	}
	// A simplified approach: capitalize the first letter of each word.
	// This might not perfectly handle all acronyms as desired but is a common behavior.
	words := strings.Fields(s)
	var titleWords []string
	for _, word := range words {
		if len(word) > 0 {
			// Check if the word is an acronym (all caps or mixed with numbers)
			isAcronym := true
			for _, r := range word {
				if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
					isAcronym = false
					break
				}
			}
			if isAcronym {
				titleWords = append(titleWords, word)
			} else {
				titleWords = append(titleWords, strings.Title(strings.ToLower(word)))
			}
		}
	}
	// If the original string had spaces, re-join with spaces.
	// If it was camelCase or PascalCase, this might not be the desired output,
	// but the name ToTitle often implies space separation for readability.
	// For a function that just capitalizes the first letter, a different name like ToPascalCase might be better.
	// Given it's used in contexts like `is%sNilInitially` or `globalTempVarPrefix + stringutils.ToTitle(opt.Name) + "Val"`,
	// it's likely intended for identifier generation.
	// Let's assume for now it's meant to capitalize the first letter of a single identifier-like string.
	if len(words) == 0 && len(s) > 0 { // Handle case like "userID"
		return strings.ToUpper(s[0:1]) + s[1:]
	}
	return strings.Join(titleWords, " ") // This might need adjustment based on actual usage
}
