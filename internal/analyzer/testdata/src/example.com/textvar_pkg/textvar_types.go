package textvar_pkg

// Type with value receiver for marshaling, pointer for unmarshaling
type MyTextValue string

func (m MyTextValue) MarshalText() ([]byte, error) {
	return []byte(string(m)), nil
}

func (m *MyTextValue) UnmarshalText(text []byte) error {
	*m = MyTextValue(text)
	return nil
}

// Type with pointer receivers for both
type MyPtrTextValue struct {
	Value string
}

func (m *MyPtrTextValue) MarshalText() ([]byte, error) {
	return []byte(m.Value), nil
}

func (m *MyPtrTextValue) UnmarshalText(text []byte) error {
	m.Value = string(text)
	return nil
}

type MyOnlyUnmarshaler string

func (m *MyOnlyUnmarshaler) UnmarshalText(text []byte) error {
	*m = MyOnlyUnmarshaler(text)
	return nil
}

type MyOnlyMarshaler string

func (m MyOnlyMarshaler) MarshalText() ([]byte, error) {
	return []byte(string(m)), nil
}

type TextVarOptions struct {
	FieldA MyTextValue         // Should be Unmarshaler (via *MyTextValue) & Marshaler (via MyTextValue)
	FieldB *MyPtrTextValue     // Should be Unmarshaler & Marshaler (both via *MyPtrTextValue)
	FieldC MyPtrTextValue      // Should be Unmarshaler & Marshaler (both via *MyPtrTextValue, on value type)
	FieldD string              // Standard string, neither
	FieldE *MyTextValue        // Pointer to type from FieldA
	FieldF MyOnlyUnmarshaler   // Only Unmarshaler
	FieldG MyOnlyMarshaler     // Only Marshaler
}
