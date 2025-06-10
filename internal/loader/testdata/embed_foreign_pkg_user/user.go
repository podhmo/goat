package userpkg

import (
	"example.com/embed_foreign_pkg_base" // This path will be resolved by testdataLocator
)

// UserStruct embeds BaseStruct from another package.
type UserStruct struct {
	Name string `json:"name"`
	embed_foreign_pkg_base.BaseStruct
	OwnField  string `json:"own_field"`
	AnotherID int    `json:"another_id" custom_tag:"custom_value"`
}

type AnotherUserStruct struct {
	Data string
}
