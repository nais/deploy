package deployment

import (
	"crypto/hmac"
	"crypto/sha512"
	"errors"
	"github.com/golang/protobuf/proto"
)

var ErrSignaturesDiffer = errors.New("signatures differ")

func SignMessage(msg proto.Message, key string) (*SignedMessage, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	hasher := hmac.New(sha512.New, []byte(key))
	hasher.Write(payload)
	sum := hasher.Sum(nil)

	return &SignedMessage{
		Message:   payload,
		Signature: sum,
	}, nil
}

func CheckMessageSignature(msg SignedMessage, key string) error {
	hasher := hmac.New(sha512.New, []byte(key))
	hasher.Write(msg.Message)
	sum := hasher.Sum(nil)

	if !hmac.Equal(sum, msg.Signature) {
		return ErrSignaturesDiffer
	}

	return nil
}
