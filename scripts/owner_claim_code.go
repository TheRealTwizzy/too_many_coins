package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const ownerClaimWindowSeconds int64 = 300
const ownerClaimAppSalt = "tmc-admin-claim-v1"
const ownerClaimCodeLength = 8

func main() {
	secret := strings.TrimSpace(os.Getenv("OWNER_CLAIM_SECRET"))
	if secret == "" {
		fmt.Println("OWNER_CLAIM_SECRET is required")
		os.Exit(1)
	}

	now := time.Now().UTC()
	windowIndex := now.Unix() / ownerClaimWindowSeconds
	code, err := ownerClaimCodeForWindow(secret, windowIndex)
	if err != nil {
		fmt.Println("Failed to generate claim code")
		os.Exit(1)
	}
	remaining := ownerClaimWindowSecondsRemaining(now)

	fmt.Printf("Owner claim code: %s\n", code)
	fmt.Printf("Window rotates in: %ds\n", remaining)
}

func ownerClaimCodeForWindow(secret string, windowIndex int64) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	message := []byte(ownerClaimAppSalt + ":" + strconv.FormatInt(windowIndex, 10))
	if _, err := mac.Write(message); err != nil {
		return "", err
	}
	sum := mac.Sum(nil)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:5])
	encoded = strings.ToUpper(encoded)
	if len(encoded) < ownerClaimCodeLength {
		encoded = encoded + strings.Repeat("A", ownerClaimCodeLength-len(encoded))
	}
	encoded = encoded[:ownerClaimCodeLength]
	return encoded[:4] + "-" + encoded[4:], nil
}

func ownerClaimWindowSecondsRemaining(now time.Time) int64 {
	elapsed := now.UTC().Unix() % ownerClaimWindowSeconds
	if elapsed == 0 {
		return 0
	}
	return ownerClaimWindowSeconds - elapsed
}
