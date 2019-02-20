package config

type Config struct {
	LogFormat    string
	LogLevel     string
	KafkaBrokers []string
	KafkaTopic   string
}

func DefaultConfig() *Config {
	return &Config{
		LogFormat:    "text",
		LogLevel:     "debug",
		KafkaBrokers: []string{"localhost:9092"},
		KafkaTopic:   "deployments",
	}
}
