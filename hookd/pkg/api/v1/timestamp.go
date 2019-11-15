package api_v1

import (
	"fmt"
	"math"
	"time"
)

type Timestamp int64

func (t Timestamp) Validate() error {
	diff := int64(t) - time.Now().Unix()
	if math.Abs(float64(diff)) > MaxTimeSkew {
		return fmt.Errorf("request is not within allowed timeframe")
	}
	return nil
}
