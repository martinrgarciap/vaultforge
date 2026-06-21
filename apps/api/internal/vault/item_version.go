package vault

const InitialItemVersion = 1

func ValidateExpectedItemVersion(version int) error {
	if version < InitialItemVersion {
		return ErrItemVersionInvalid
	}

	return nil
}
