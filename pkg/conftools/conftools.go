package conftools

import (
	"fmt"
	"sort"

	"github.com/mitchellh/mapstructure"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func decoderHook(dc *mapstructure.DecoderConfig) {
	dc.TagName = "json"
	dc.ErrorUnused = true
}

func Load(cfg interface{}) error {
	var err error

	err = viper.ReadInConfig()
	if err != nil {
		if err.(viper.ConfigFileNotFoundError) != err {
			return err
		}
	}

	flag.Parse()

	err = viper.BindPFlags(flag.CommandLine)
	if err != nil {
		return err
	}

	err = viper.Unmarshal(cfg, decoderHook)
	if err != nil {
		return err
	}

	return nil
}

// Return a human-readable printout of all configuration options, except secret stuff.
func Format(disallowedKeys []string) []string {
	ok := func(key string) bool {
		for _, forbiddenKey := range disallowedKeys {
			if forbiddenKey == key {
				return false
			}
		}
		return true
	}

	var keys sort.StringSlice = viper.AllKeys()

	printed := make([]string, 0)

	keys.Sort()
	for _, key := range keys {
		if ok(key) {
			printed = append(printed, fmt.Sprintf("%s: %v", key, viper.Get(key)))
		} else {
			printed = append(printed, fmt.Sprintf("%s: ***REDACTED***", key))
		}
	}

	return printed
}
