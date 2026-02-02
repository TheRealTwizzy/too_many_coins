package main

import "unicode"

func isValidPlayerID(playerID string) bool {
	if playerID == "" || len(playerID) > 64 {
		return false
	}

	for _, r := range playerID {
		if r == '-' || r == '_' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}

	return true
}
