# NAIS deployment

## Overview
NAIS deployment facilitates application deployment into NAV's Kubernetes clusters using the Github Deployments API.

Developers push or merge code into the master branch of a Git repository, triggering an automated build using CircleCI, Travis CI, or Jenkins.
A successful build produces a Docker image artifact, which is uploaded onto Docker Hub.
The final step in the build pipeline triggers the Github Deployments API, where NAIS deployment hooks in, deploying the application on Kubernetes.

![Sequence diagram of deployment components](doc/sequence.png)

## How it works
1. As the final step in one of your CI pipelines, a [deployment request](https://developer.github.com/v3/repos/deployments/#create-a-deployment) is created using GitHub's API. This will trigger a webhook set up at Github.
2. `hookd` receives the webhook, verifies its integrity and authenticity, and passes the message on to `deployd` via Kafka.
3. `deployd` receives the message from `hookd`, assumes the identity of the deploying team, and applies your _Kubernetes resources_ into the specified [cluster](https://doc.nais.io/clusters).
4. If the Kubernetes resources contained any _Application_ or _Deployment_ resources, `deployd` will wait until these are rolled out successfully, or a timeout occurs.

Any fatal error will short-circuit the process with a `error` or `failure` status posted back to Github. A successful deployment will result in a `success` status.
Intermediary statuses might be posted, indicating the current state of the deployment.

## Usage

The usage documentation has been moved to [NAIS platform documentation](https://doc.nais.io/deployment).

### Deploy API

Post to `/api/v1/deploy` to deploy one or more resources into one of our Kubernetes clusters.

Successful requests result in creation of a _deployment_ object on GitHub. Use this object
to track the status of your deployment.

#### Request specification

```json
{
  "resources": [
    {
      "kind": "Application",
      "apiVersion": "nais.io/v1alpha1",
      "metadata": { ... },
      "spec": { ... },
    }
  ],
  "team": "nobody",
  "cluster": "local",
  "environment": "dev-fss:default",
  "owner": "navikt",
  "repository": "deployment",
  "ref": "master",
  "timestamp": 1572942789,
}
```

| Field | Type | Description |
|-------|------|-------------|
| resources | list[object] | Array of Kubernetes resources |
| team | string | Team tag |
| cluster | string | Kubernetes cluster, see [NAIS clusters](https://doc.nais.io/clusters) |
| environment | string | GitHub environment |
| owner | string | GitHub repository owner |
| repository | string | GitHub repository name |
| ref | string | GitHub commit hash or tag |
| timestamp | int64 | Current Unix timestamp |

Additionally, the header `X-NAIS-Signature` must contain a keyed-hash message authentication code (HMAC).
The code can be derived by hashing the request body using the SHA256 algorithm together with your team's NAIS Deploy API key.

#### Response specification

```json
{
  "logURL": "http://localhost:8080/logs?delivery_id=9a0d1702-e7c5-448f-8a90-1e5ee29a043b&ts=1572437924",
  "correlationID": "9a0d1702-e7c5-448f-8a90-1e5ee29a043b",
  "message": "successful deployment",
  "githubDeployment": { ... }
}
```

| Field | Type | Description |
|-------|------|-------------|
| logURL | string | Direct link to human readable frontend where logs for this specific deployment can be read |
| correlationID | string | UUID used for correlation tracking across systems, especially in logs |
| message | string | Human readable indication of API result |
| githubDeployment | object | [Data returned from GitHub Deployments API](https://developer.github.com/v3/repos/deployments/#get-a-single-deployment) |

#### Response status codes

| Code | Retriable | Description |
|-------|------|-------------|
| 201 | N/A | The request was valid and will be deployed. Track the status of your deployment using the GitHub Deployments API. |
| 400 | NO | The request contains errors and cannot be processed. Check the `message` field for details.
| 403 | MAYBE | Authentication failed. Check that you're supplying the correct `team`; that the team is present on GitHub and has admin access to your repository; that you're using the correct API key; and properly HMAC signing the request. |
| 404 | NO | Wrong URL. |
| 5xx | YES | NAIS deploy is having problems and is currently being fixed. Retry later. |


## Application components

### hookd
This service communicates with Github, and acts as a relay between the Internet and our Kubernetes clusters.

Its main tasks are to:
* validate deployment events
* relay deployment requests to _deployd_ using Kafka
* report deployment status back to GitHub

The validation part is done by checking if the signature attached to the deployment event is valid, and by checking the format of the deployment.
Refer to the [GitHub documentation](https://developer.github.com/webhooks/securing/) as to how webhooks are secured.

### deployd
Deployd's responsibility is to deploy resources into a Kubernetes cluster, and report state changes back to hookd using Kafka.

### token-generator
token-generator is a daemon that can issue credentials out-of-band. For example:

#### Overview
```
> POST /api/v1/tokens
> Authorization: Basic YWRtaW46YWRtaW4=
> {
>     "repository": "navikt/deployment",
>     "sources": ["github"],
>     "sinks": ["circleci"]
> }
---
< HTTP/1.1 201 Created
< X-Correlation-Id: 59b7112f-2c50-4a69-acfd-775f8a14b3bc
< Date: Wed, 31 Jul 2019 13:17:26 GMT
< Content-Length: 0
```

This request will issue a _JSON Web Token_ (JWT) on behalf of a Github App Installation.
~The token will be scoped to the specific repository in question.~
(See [go-github #1238](https://github.com/google/go-github/pull/1238))
Afterwards, the token is uploaded out-of-band to the CircleCI build belonging to this repository.
The token is valid for one hour and is available as the environment variable `$GITHUB_TOKEN`.
The client sees only the HTTP response. The token itself is available to CircleCI jobs.
Calls to this API will block, returning only when either the token has been uploaded,
or when an error occurred.

If you run into trouble, you can search the logs for `correlation-id:"..."`.

#### User portal

Users can log in with their Azure credentials in the user portal at `http://localhost:8080`.

In the future, the user portal will enable users to provision API keys to their team.

#### Configuration

Create a file `token-generator.yaml` with the following contents and place it in your working directory.

```yaml
# for the 'github' source
github:
  appid: 246
  installid: 753
  keyfile: github-app-private-key.pem

# for the 'circleci' sink
circleci:
  apitoken: 6d9627000451337133713371337a72b40c55a47f

# optional; for the user portal.
azure:
  tenant: 39273030-3046-400d-9ee4-34d916ecdc97
  clientid: 306b4e93-3987-4c45-ae38-d16e403e0144
  clientsecret: eW91d2lzaAo=
  redirecturl: http://localhost:8080/auth/callback
  discoveryurl: https://login.microsoftonline.com/39273030-3046-400d-9ee4-34d916ecdc97/discovery/keys

# optional; for Google Cloud Storage backed API key storage.
storage:
  bucketname: your-gcs-bucket
  keyfile: your-credentials.json
```

The same configuration can be accessed using flags or environment variables:

```
./token-generator --help

export GITHUB_APPID=246
export CIRCLECI_APITOKEN=6d9627000451337133713371337a72b40c55a47f
./token-generator --github.installid=753 --github.keyfile=github-app-private-key.pem
```

### Kafka
Kafka is used as a communication channel between hookd and deployd. Hookd sends deployment requests to a `deploymentRequests` topic, which fans out
and in turn hits all the deployd instances. Deployd acts on the information, and then sends a deployment status to the `deploymentStatus` topic.
Hookd picks up replies to this topic, and publishes the deployment status to Github.

### Amazon S3 (Amazon Simple Storage Service)
Used as a configuration backend. Information about repository team access is stored here, and accessed on each deployment request.


## Developing

### Compiling
[Install Golang 1.12 or newer](https://golang.org/doc/install).

Check out the repository and run `make`. Dependencies will download automatically, and you should have three binary files at `hookd/hookd`, `deployd/deployd` and `token-generator`.

### External dependencies
Start the external dependencies by running `docker-compose up`. This will start local Kafka, S3, and Vault servers.

The S3 access and secret keys are `accesskey` and `secretkey` respectively. Conveniently, these are
the default options for _hookd_ as well, so you don't have to configure anything.

### Vault
The `/api/v1/deploy` endpoint uses Hashicorp Vault to store teams' API keys. To make this work, set up a Vault KV store version 1
on the server specified by `--vault-address` and on the path specified by `--vault-path`.

For instance, with the default values of `--vault-address=http://localhost:8080 --vault-path=/v1/apikey/nais-deploy --vault-key=key`:

* Set up `/apikey` as a KV v1 store.
* Create secrets under `/apikey/nais-deploy/<team>` with key `key` and the pre-shared secret as the value.

### token-generator
* Set up a google cloud storage bucket
* Create a credentials file

### Development on the frontend application
To enable the frontend on your local instance, you need to configure hookd against the staging deployment application at Github.
This is required to enable OAuth and Github queries.
The parameters can be found on the [Github installation page](https://github.com/organizations/navikt/settings/installations/).
You must also generate a private key for this installation, in order to sign your JSON web tokens.

Configure hookd as follows:

```
--github-enabled=true \
--github-install-id=XXXXXX \
--github-app-id=XXXXXX \
--github-client-id=XXXXXX \
--github-client-secret=XXXXXX \
--github-key-file=/path/to/private-key.pem \
```

### Simulating Github deployment requests
When you want to send webhooks to _hookd_ without invoking Github, you can use the `mkdeploy` tool, which simulates these requests.

Start a local Kafka instance as described above, and then run your local hookd instance.
```
./hookd/hookd
```

If you want to deploy, you want to start up `deployd` as well:
```
./deployd/deployd
```

Compile the `mkdeploy` tool:
```
cd hookd/cmd/mkdeploy
make
```

You can now run `mkdeploy`. The default parameters should work fine, but you'll probably want to specify a deployment payload, which, by default, is empty.
Run `./mkdeploy --help` to see which options you can tweak.
