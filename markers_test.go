package goat

import (
	"reflect"
	"testing"
)

// Test for Enum marker function
func TestEnum(t *testing.T) {
	testCases := []struct {
		name     string
		input    []any // Using []any for broader testability, though T is specific at call site
		expected []any
	}{
		{
			name:     "string slice",
			input:    []any{"alpha", "beta", "gamma"},
			expected: []any{"alpha", "beta", "gamma"},
		},
		{
			name:     "int slice",
			input:    []any{1, 2, 3},
			expected: []any{1, 2, 3},
		},
		{
			name:     "empty slice",
			input:    []any{},
			expected: []any{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result []any // Adapt based on type of tc.input if needed, or keep generic
			switch v := tc.input.(type) {
			case []any: // This path matches our test case structure
				// To call the generic Enum, we need a typed slice.
				// This test primarily checks if Enum returns its input, type conversion is for test setup.
				if len(v) > 0 {
					switch v[0].(type) {
					case string:
						typedInput := make([]string, len(v))
						for i, item := range v {
							typedInput[i] = item.(string)
						}
						resultTyped := Enum(typedInput)
						result = make([]any, len(resultTyped))
						for i, item := range resultTyped {
							result[i] = item
						}
					case int:
						typedInput := make([]int, len(v))
						for i, item := range v {
							typedInput[i] = item.(int)
						}
						resultTyped := Enum(typedInput)
						result = make([]any, len(resultTyped))
						for i, item := range resultTyped {
							result[i] = item
						}
					default:
						if v == nil || len(v) == 0 { // Handle empty or nil explicitly
							result = Enum(v) // Call with []any if it works or specific typed nil
						} else {
							t.Skipf("Test setup for type %T not fully implemented for Enum test", v[0])
						}
					}
				} else { // empty or nil slice
					if v == nil {
						result = Enum[any](nil) // Explicitly type for nil
					} else {
						result = Enum(v) // for empty []any{}
					}
				}

			default:
				if tc.input == nil {
					result = Enum[any](nil) // Test with nil explicitly typed
				} else {
					t.Fatalf("Unsupported input type for Enum test: %T", tc.input)
				}
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Enum(%v) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// Test for Default marker function
func TestDefault(t *testing.T) {
	testCases := []struct {
		name           string
		defaultValue   any
		enumConstraint [][]any // Outer slice for varargs, inner for the actual enum values
		expectedReturn any
	}{
		{
			name:           "string default no enum",
			defaultValue:   "hello",
			enumConstraint: nil,
			expectedReturn: "hello",
		},
		{
			name:           "int default no enum",
			defaultValue:   42,
			enumConstraint: nil,
			expectedReturn: 42,
		},
		{
			name:           "bool default no enum",
			defaultValue:   true,
			enumConstraint: nil,
			expectedReturn: true,
		},
		{
			name:           "string default with string enum",
			defaultValue:   "one",
			enumConstraint: [][]any{{"one", "two", "three"}},
			expectedReturn: "one",
		},
		{
			name:           "int default with int enum",
			defaultValue:   10,
			enumConstraint: [][]any{{10, 20, 30}},
			expectedReturn: 10,
		},
		{
			name:           "string default with empty enum",
			defaultValue:   "test",
			enumConstraint: [][]any{{}}, // Empty enum constraint
			expectedReturn: "test",
		},
		{
			name:           "string default with nil enum constraint (varargs not passed)",
			defaultValue:   "test_nil_constraint",
			enumConstraint: nil, // equivalent to not passing the vararg
			expectedReturn: "test_nil_constraint",
		},
		{
			name:           "string default with nil actual enum slice",
			defaultValue:   "test_nil_slice",
			enumConstraint: [][]any{nil}, // Varargs passed, but the slice itself is nil
			expectedReturn: "test_nil_slice",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result any
			// Helper to call generic Default with specific types for testing
			callDefault := func(dv any, ec [][]any) any {
				switch val := dv.(type) {
				case string:
					var typedEc [][]string
					if ec != nil {
						typedEc = make([][]string, len(ec))
						for i, subSlice := range ec {
							if subSlice != nil {
								typedEc[i] = make([]string, len(subSlice))
								for j, item := range subSlice {
									typedEc[i][j] = item.(string)
								}
							}
						}
					}
					if len(typedEc) > 0 {
						return Default(val, typedEc[0])
					}
					return Default(val)
				case int:
					var typedEc [][]int
					if ec != nil {
						typedEc = make([][]int, len(ec))
						for i, subSlice := range ec {
							if subSlice != nil {
								typedEc[i] = make([]int, len(subSlice))
								for j, item := range subSlice {
									typedEc[i][j] = item.(int)
								}
							}
						}
					}
					if len(typedEc) > 0 {
						return Default(val, typedEc[0])
					}
					return Default(val)
				case bool:
					// Enum for bool is less common but testable
					var typedEc [][]bool
					if ec != nil {
						typedEc = make([][]bool, len(ec))
						for i, subSlice := range ec {
							if subSlice != nil {
								typedEc[i] = make([]bool, len(subSlice))
								for j, item := range subSlice {
									typedEc[i][j] = item.(bool)
								}
							}
						}
					}
					if len(typedEc) > 0 {
						return Default(val, typedEc[0])
					}
					return Default(val)
				default:
					t.Fatalf("Unsupported type for Default test: %T", dv)
					return nil
				}
			}

			result = callDefault(tc.defaultValue, tc.enumConstraint)

			if !reflect.DeepEqual(result, tc.expectedReturn) {
				t.Errorf("Default(%v, %v) = %v, want %v", tc.defaultValue, tc.enumConstraint, result, tc.expectedReturn)
			}
		})
	}
}
