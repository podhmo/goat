package simplepkg

import (
	"fmt"

	"example.com/anotherpkg"
)

type MyStruct struct {
	Name      string
	Age       int
	OtherData anotherpkg.AnotherStruct
}

func (ms *MyStruct) Greet() string {
	return fmt.Sprintf("Hello, my name is %s, I am %d years old, and I have other data: %s.", ms.Name, ms.Age, ms.OtherData.Value)
}
