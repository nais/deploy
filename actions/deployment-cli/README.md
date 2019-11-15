# nais deploy action

Github Action used for performing NAIS deployments.

| input	     | description                                                         | required |
|------------|-------------------------------------------------------------------- |----------|
| cluster    | cluster to deploy to                                                | true     |
| repository | respository to create deployment                                    | true     |
| team       | which team this deploy is for                                       | true     |
| resource   | path to Kubernetes resource to apply                                | true     | 
| vars       | path to JSON file containing variables used when executing template | false    | 

## example workflow

```
name: stuff and deploy
on: [push]
jobs:
  build:
    name: build
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v1 
    - name: deploy to dev
      uses: nais/deploy/actions/deployment-cli@master
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      with:
        cluster: dev
        repository: my/repo
        team: a
        resource: nais.yaml 
        vars: config/dev.json
```
