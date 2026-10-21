module github.com/nais/deploy

go 1.27.3

tool (
	github.com/securego/gosec/v2/cmd/gosec
	github.com/sethvargo/ratchet
	golang.org/x/tools/cmd/deadcode
	golang.org/x/vuln/cmd/govulncheck
	honnef.co/go/tools/cmd/staticcheck
	mvdan.cc/gofumpt
)

require (
	github.com/aymerick/raymond v2.0.2+incompatible
	github.com/nais/api/pkg/apiclient v0.0.0-20261021062219-2a0b93df125d
	github.com/sirupsen/logrus v1.10.2
	github.com/stretchr/testify v1.12.1
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v2 v2.4.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.24.1 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/anthropics/anthropic-sdk-go v1.84.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/braydonk/yaml v0.9.0 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/ccojocar/zxcvbn-go v1.0.4 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/docker/cli v29.9.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.12 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.22.1 // indirect
	github.com/google/go-github/v89 v89.0.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/s2a-go v0.1.11 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.23 // indirect
	github.com/googleapis/gax-go/v2 v2.26.2 // indirect
	github.com/gookit/color v1.6.3 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/openai/openai-go/v3 v3.84.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pb33f/go-yaml v0.1.1 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.2 // indirect
	github.com/securego/gosec/v2 v2.29.0 // indirect
	github.com/sethvargo/ratchet v0.12.0 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tidwall/gjson v1.20.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.2 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20260212183809-81e46e3db34a // indirect
	golang.org/x/exp/typeparams v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261020154912-7bf0a11256d5 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
	google.golang.org/api v0.306.0 // indirect
	google.golang.org/genai v1.75.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b // indirect
	honnef.co/go/tools v0.8.2 // indirect
	mvdan.cc/gofumpt v0.12.0 // indirect
)
