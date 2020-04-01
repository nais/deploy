package api_v1

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Key []byte

func (k Key) String() string {
	return hex.EncodeToString(k)
}

func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

func (k *Key) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	_, err = hex.DecodeString(str)
	if err != nil {
		return fmt.Errorf("expecting hex string: %s", err)
	}
	return nil
}
