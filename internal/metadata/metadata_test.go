package metadata

import "testing"

func TestCommandMetadata(t *testing.T) {
	t.Run("create command metadata", func(t *testing.T) {
		name := "test-command"
		md := CommandMetadata{
			Name: name,
		}

		if md.Name != name {
			t.Errorf("expected Name to be %q, but got %q", name, md.Name)
		}
	})
}
