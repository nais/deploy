package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/nais/deploy/pkg/crypto"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

const (
	defaultEncryptionKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
)

var (
	shouldEncrypt = flag.Bool("encrypt", false, "try encrypting input data")
	shouldDecrypt = flag.Bool("decrypt", false, "try decrypting input data")
	encryptionKey = flag.String("key", getEnvDefault("ENCRYPTION_KEY", defaultEncryptionKey), "encryption key")
	useHex        = flag.Bool("hex", true, "output data as hex string")
)

func getEnvDefault(env string, def string) string {
	val, found := os.LookupEnv(env)
	if !found {
		return def
	}
	return val
}

func decrypt(s string, key []byte) (string, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	decrypted, err := crypto.Decrypt(decoded, key)
	if err != nil {
		return "", err
	}
	if *useHex {
		return hex.EncodeToString(decrypted), nil
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
