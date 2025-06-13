package customtypes

// MyEnum is a custom type, potentially an alias for string or a distinct type.
// For testing constants, it's often defined as a string type for easy verification.
type MyEnum string

const EnumValA MyEnum = "val-a"
const EnumValB MyEnum = "val-b"
const EnumValC MyEnum = "val-c"

const NotStringConst int = 123

// MyCustomEnumValues is used by existing tests, ensure it's compatible.
// It seems to expect a slice of strings, derived from MyEnum constants.
var MyCustomEnumValues = []string{string(EnumValA), string(EnumValB), string(EnumValC)}

// Different set of constants for variety in tests if needed
const AnotherValX MyEnum = "another-x"
const AnotherValY MyEnum = "another-y"
