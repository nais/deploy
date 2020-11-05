package pb

import (
	"crypto/hmac"
	"crypto/sha512"
	"fmt"

	"github.com/golang/protobuf/proto"
)

func mksum(payload, key []byte) []byte {
	hasher := hmac.New(sha512.New, key)
	hasher.Write(payload)
	return hasher.Sum(nil)
}

func signMessage(payload []byte, key string) ([]byte, error) {
	sum := mksum(payload, []byte(key))

	signed := &SignedMessage{
		Message:   payload,
		Signature: sum,
	}

	return proto.Marshal(signed)
}

func checkMessageSignature(msg SignedMessage, key string) error {
	sum := mksum(msg.Message, []byte(key))

	if !hmac.Equal(sum, msg.Signature) {
		return fmt.Errorf("signatures differ")
	}

	return nil
}

// WrapMessage signs and encodes a protobuf message.
func WrapMessage(msg proto.Message, key string) ([]byte, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("while encoding Protobuf: %s", err)
	}

	return signMessage(payload, key)
}

// UnwrapMessage decodes a signed protobuf message, validates the message signature,
// and returns the typed Protobuf message if the validation succeeds.
func UnwrapMessage(msg []byte, key string, dest proto.Message) error {
	wrapped := SignedMessage{}
	if err := proto.Unmarshal(msg, &wrapped); err != nil {
		return fmt.Errorf("while decoding Protobuf: %s", err)
	}

	if err := checkMessageSignature(wrapped, key); err != nil {
		return err
	}

	if err := proto.Unmarshal(wrapped.Message, dest); err != nil {
		return fmt.Errorf("while decoding inner Protobuf: %s", err)
	}

	return nil
}
