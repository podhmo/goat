package customtypes

type MyCustomEnum string

const (
	OptionX MyCustomEnum = "option-x"
	OptionY MyCustomEnum = "option-y"
	OptionZ MyCustomEnum = "option-z"
)

func GetCustomEnumOptions() []MyCustomEnum {
	return []MyCustomEnum{OptionX, OptionY, OptionZ}
}

var MyCustomEnumValues = []string{
	string(OptionX),
	string(OptionY),
	string(OptionZ),
}
