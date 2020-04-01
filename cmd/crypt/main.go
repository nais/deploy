package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/navikt/deployment/pkg/crypto"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	shouldEncrypt = flag.Bool("encrypt", false, "try encrypting input data")
	shouldDecrypt = flag.Bool("decrypt", false, "try decrypting input data")
	encryptionKey = flag.String("key", os.Getenv("ENCRYPTION_KEY"), "encryption key")
)

func decrypt(s string, key []byte) (string, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	decrypted, err := crypto.Decrypt(decoded, key)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

func encrypt(s string, key []byte) (string, error) {
	encrypted, err := crypto.Encrypt([]byte(s), key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

func run() error {
	errors := 0

	flag.Parse()

	if len(*encryptionKey) == 0 {
		return fmt.Errorf("ENCRYPTION_KEY environment variable not provided")
	}
	key, err := crypto.KeyFromHexString(*encryptionKey)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimRight(scanner.Text(), "\n\r")
		if *shouldDecrypt {
			decrypted, err := decrypt(text, key)
			if err == nil {
				fmt.Println(decrypted)
			} else {
				log.Errorf("decryption failed: %s", err)
				errors++
			}
		}
		if *shouldEncrypt {
			encrypted, err := encrypt(text, key)
			if err == nil {
				fmt.Println(encrypted)
			} else {
				log.Errorf("encryption failed: %s", err)
				errors++
			}
		}
	}

	if errors == 0 {
		return nil
	}

	return fmt.Errorf("%d errors", errors)
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
