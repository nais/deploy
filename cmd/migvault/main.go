// migvault is a migration tool for migrating API keys from Vault into a postgresql database.
//
// prerequisites:
//
// * Vault access through navtunnel proxy
// * PostgreSQL access
// * teams.yml file from the navikt/teams repository
//
// you must configure all of these:
//
// export AZURE_CLIENT_ID=
// export AZURE_CLIENT_SECRET=
// export AZURE_DISCOVERY_URL=
// export AZURE_TEAM_MEMBERSHIP_APP_ID=
// export AZURE_TENANT=
// export POSTGRESQL_URL=
// export VAULT_ADDRESS=
// export VAULT_AUTH_PATH=
// export VAULT_AUTH_ROLE=
// export VAULT_CREDENTIALS_FILE=
// export TEAMS_YAML_FILE=
//
// run migvault:
//
// go build -o migvault *.go && ./migvault

package main

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/navikt/deployment/pkg/crypto"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/proxy"

	"github.com/navikt/deployment/hookd/pkg/azure/graphapi"
	"github.com/navikt/deployment/hookd/pkg/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/navikt/deployment/hookd/pkg/database"
)

type Config struct {
	CredentialsFile string
	Token           string
	Address         string
	Path            string
	AuthPath        string
	AuthRole        string
	KeyName         string
	PostgresURL     string
	TeamsYamlFile   string
	EncryptionKey   string
	Azure           config.Azure
}

type Team struct {
	Name string
}

type TeamsYaml struct {
	Teams []Team
}

const (
	socksProxyAddress = "localhost:14122" // navtunnel
)

var cfg = Config{
	CredentialsFile: getEnv("VAULT_CREDENTIALS_FILE", ""),
	Address:         getEnv("VAULT_ADDRESS", "http://localhost:8200"),
	KeyName:         getEnv("VAULT_KEY_NAME", "key"),
	Path:            getEnv("VAULT_PATH", "/v1/apikey/nais-deploy"),
	AuthPath:        getEnv("VAULT_AUTH_PATH", "/v1/auth/kubernetes/login"),
	AuthRole:        getEnv("VAULT_AUTH_ROLE", ""),
	Token:           getEnv("VAULT_TOKEN", "123456789"),
	PostgresURL:     getEnv("POSTGRESQL_URL", "postgres://postgres:root@localhost/hookd"),
	TeamsYamlFile:   getEnv("TEAMS_YAML_FILE", "/dev/null"),
	EncryptionKey:   getEnv("ENCRYPTION_KEY", "ab54620542594320965429432097564295324013249013209175aabbccddeefa"),
	Azure: config.Azure{
		ClientID:            getEnv("AZURE_CLIENT_ID", ""),
		ClientSecret:        getEnv("AZURE_CLIENT_SECRET", ""),
		Tenant:              getEnv("AZURE_TENANT", ""),
		DiscoveryURL:        getEnv("AZURE_DISCOVERY_URL", "https://login.microsoftonline.com/common/discovery/v2.0/keys"),
		TeamMembershipAppID: getEnv("AZURE_TEAM_MEMBERSHIP_APP_ID", ""),
	},
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	flag.StringVar(&cfg.Path, "vault-path", cfg.Path, "Base path to Vault KV API key store.")
	flag.StringVar(&cfg.AuthPath, "vault-auth-path", cfg.AuthPath, "Path to Vault authentication endpoint.")
	flag.StringVar(&cfg.AuthRole, "vault-auth-role", cfg.AuthRole, "Role used for Vault authentication.")
	flag.StringVar(&cfg.Address, "vault-address", cfg.Address, "Address to Vault server.")
	flag.StringVar(&cfg.KeyName, "vault-key-name", cfg.KeyName, "API keys are stored in this key.")
	flag.StringVar(&cfg.CredentialsFile, "vault-credentials-file", cfg.CredentialsFile, "Credentials for authenticating against Vault retrieved from this file (overrides --vault-token).")
	flag.StringVar(&cfg.Token, "vault-token", cfg.Token, "Vault static token.")
	flag.StringVar(&cfg.PostgresURL, "postgresql-url", cfg.PostgresURL, "postgresql url")
	flag.StringVar(&cfg.TeamsYamlFile, "teams-yaml-file", cfg.TeamsYamlFile, "path to teams.yaml from navikt/teams repository")
	flag.StringVar(&cfg.EncryptionKey, "encryption-key", cfg.EncryptionKey, "encryption key for team keys")
	flag.StringVar(&cfg.Azure.ClientID, "azure.clientid", cfg.Azure.ClientID, "Azure ClientId.")
	flag.StringVar(&cfg.Azure.ClientSecret, "azure.clientsecret", cfg.Azure.ClientSecret, "Azure ClientSecret")
	flag.StringVar(&cfg.Azure.DiscoveryURL, "azure.discoveryurl", cfg.Azure.DiscoveryURL, "Azure DiscoveryURL")
	flag.StringVar(&cfg.Azure.Tenant, "azure.tenant", cfg.Azure.Tenant, "Azure Tenant")
	flag.StringVar(&cfg.Azure.TeamMembershipAppID, "azure.teamMembershipAppID", cfg.Azure.TeamMembershipAppID, "Application ID of canonical team list")

	flag.Parse()

	dialer, err := proxy.SOCKS5("tcp", socksProxyAddress, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("proxy: %s", err)
	}

	transport := &http.Transport{}
	transport.Dial = dialer.Dial
	httpClient := &http.Client{
		Transport: transport,
	}

	apiKeys := &VaultApiKeyStorage{
		Address:    cfg.Address,
		Path:       cfg.Path,
		AuthPath:   cfg.AuthPath,
		AuthRole:   cfg.AuthRole,
		KeyName:    cfg.KeyName,
		Token:      cfg.Token,
		Refreshed:  make(chan interface{}, 1300),
		HttpClient: httpClient,
	}

	if len(cfg.CredentialsFile) > 0 {
		credentials, err := ioutil.ReadFile(cfg.CredentialsFile)
		if err != nil {
			log.Fatalf("read Vault token file: %s", err)
		}
		apiKeys.Credentials = string(credentials)
		apiKeys.Token = ""

		go apiKeys.RefreshLoop()
		log.Infof("waiting for vault token...")
		<-apiKeys.Refreshed
		log.Infof("vault token ok")
	}

	encryptionKey, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("encryption key: %s", err)
	}

	db, err := database.New(cfg.PostgresURL, encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	graphAPIClient := graphapi.NewClient(cfg.Azure)

	file, err := os.Open(cfg.TeamsYamlFile)
	if err != nil {
		log.Fatal(err)
	}
	teams := &TeamsYaml{}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(teams)
	if err != nil {
		log.Fatal(err)
	}

	for _, team := range teams.Teams {
		log.Infof("processing %s", team.Name)

		apiKey, err := apiKeys.Read(team.Name)
		if err != nil {
			log.Fatal(err)
		}

		azureTeam, err := graphAPIClient.Team(context.Background(), team.Name)
		if err != nil {
			log.Fatal(err)
		}

		encryptedKey, err := crypto.Encrypt(apiKey, encryptionKey)
		if err != nil {
			log.Fatalf("encrypting key: %s", err)
		}

		err = db.Write(team.Name, azureTeam.AzureUUID, encryptedKey)
		if err != nil {
			log.Fatal(err)
		}
	}
}
