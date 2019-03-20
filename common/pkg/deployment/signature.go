package deployment

import (
	"crypto/hmac"
	"crypto/sha512"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
)

var ErrSignaturesDiffer = errors.New("signatures differ")

func signMessage(payload []byte, key string) ([]byte, error) {
	hasher := hmac.New(sha512.New, []byte(key))
	hasher.Write(payload)
	sum := hasher.Sum(nil)

	signed := &SignedMessage{
		Message:   payload,
		Signature: sum,
	}

	return proto.Marshal(signed)
}

func checkMessageSignature(msg SignedMessage, key string) error {
	hasher := hmac.New(sha512.New, []byte(key))
	hasher.Write(msg.Message)
	sum := hasher.Sum(nil)

	if !hmac.Equal(sum, msg.Signature) {
		return ErrSignaturesDiffer
	}

	return nil
}

func WrapMessage(msg proto.Message, key string) ([]byte, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("while encoding Protobuf: %s", err)
	}

	return signMessage(payload, key)
}

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
