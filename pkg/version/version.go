package version

import (
	"fmt"
	"strconv"
	"time"
)

var (
	Revision      = "(revision unknown)"   // Git commit hash
	Date          = "(version unknown)"    // Numeric version
	BuildUnixTime = "(build time unknown)" // Time of build
)

func Version() string {
	return fmt.Sprintf("%s-%s", Date, Revision)
}

func BuildTime() (*time.Time, error) {
	tm, err := strconv.ParseInt(BuildUnixTime, 10, 64)
	if err != nil {
		return nil, err
	}
	ts := time.Unix(tm, 0)
	return &ts, nil
}
