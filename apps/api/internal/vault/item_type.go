package vault

import "errors"

type ItemType string

const (
	ItemTypeLogin               ItemType = "login"
	ItemTypeAPIKey              ItemType = "api_key"
	ItemTypeEnvironmentVariable ItemType = "environment_variable"
	ItemTypeDatabaseConnection  ItemType = "database_connection"
	ItemTypeSecureNote          ItemType = "secure_note"
)

var ErrItemTypeInvalid = errors.New("item type is invalid")

func ParseItemType(value string) (ItemType, error) {
	itemType := ItemType(value)

	if !itemType.Valid() {
		return "", ErrItemTypeInvalid
	}

	return itemType, nil
}

func (itemType ItemType) Valid() bool {
	switch itemType {
	case ItemTypeLogin,
		ItemTypeAPIKey,
		ItemTypeEnvironmentVariable,
		ItemTypeDatabaseConnection,
		ItemTypeSecureNote:
		return true

	default:
		return false
	}
}
