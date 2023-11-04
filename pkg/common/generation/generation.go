package generation

import "github.com/google/uuid"

func generateUniqueID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	sessionID := id.String()
	return sessionID, nil
}
