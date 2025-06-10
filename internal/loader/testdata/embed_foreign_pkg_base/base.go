package basepkg

// BaseStruct is a base structure to be embedded.
type BaseStruct struct {
	ID      int    `json:"id,omitempty" xml:"id,attr"`
	Version string `json:"version" xml:"version"`
}

type UnrelatedBaseStruct struct {
	Name string
}
