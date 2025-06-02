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