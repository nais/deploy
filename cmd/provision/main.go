package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/provision"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

type Config struct {
	ServerURL string
	Rotate    bool
	Secret    string
	Team      string
}

var cfg = DefaultConfig()

var help = `
provision provisions team API keys.
`

const (
	provisionAPIPath = "/api/v1/provision"
	defaultServer    = "https://deploy.nais.io"
)

type ExitCode int

const (
	ExitSuccess ExitCode = iota
	ExitFailure
)

func DefaultConfig() Config {
	return Config{
		ServerURL: defaultServer,
	}
}

func init() {
	flag.ErrHelp = fmt.Errorf(help)

	flag.StringVar(&cfg.ServerURL, "server", cfg.ServerURL, "URL to API server.")
	flag.BoolVar(&cfg.Rotate, "rotate", cfg.Rotate, "Rotate API key if it already exists.")
	flag.StringVar(&cfg.Secret, "secret", cfg.Secret, "Pre-shared secret.")
	flag.StringVar(&cfg.Team, "team", cfg.Team, "Provision API key for this team.")

	flag.Parse()

	log.SetOutput(os.Stderr)

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		TimestampFormat:        time.RFC3339Nano,
		DisableLevelTruncation: true,
	})
}

func mkpayload(w io.Writer) error {
	req := api_v1_provision.Request{
		Team:      cfg.Team,
		Rotate:    cfg.Rotate,
		Timestamp: api_v1.Timestamp(time.Now().Unix()),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(req)
}

func sign(data, key []byte) string {
	return hex.EncodeToString(api_v1.GenMAC(data, key))
}

func run() (ExitCode, error) {
	var err error

	targetURL, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return ExitFailure, fmt.Errorf("wrong format of base URL: %s", err)
	}
	targetURL.Path = provisionAPIPath

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	err = mkpayload(buf)
	if err != nil {
		return ExitFailure, err
	}

	decoded, err := hex.DecodeString(cfg.Secret)
	if err != nil {
		return ExitFailure, fmt.Errorf("API key must be a hex encoded string: %s", err)
	}
	sig := sign(buf.Bytes(), decoded)

	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)
	if err != nil {
		return ExitFailure, fmt.Errorf("internal error creating http request: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add(api_v1.SignatureHeader, fmt.Sprintf("%s", sig))

	log.Infof("Submitting provision request to %s...", targetURL.String())
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ExitFailure, err
	}

	data, _ = ioutil.ReadAll(resp.Body)
	response := &api_v1_provision.Response{}
	err = json.Unmarshal(data, response)
	if err != nil {
		response.Message = string(data)
	}

	msg := fmt.Sprintf("%s: %s", resp.Status, response.Message)

	if resp.StatusCode >= 400 {
		log.Error(msg)
		return ExitFailure, nil
	}

	log.Info(msg)
	return ExitSuccess, nil
}

func main() {
	code, err := run()
	if err != nil {
		log.Errorf("fatal: %s", err)
	}
	os.Exit(int(code))
}
