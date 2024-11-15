import * as core from '@actions/core'
import {spaSetupTask, validateInputs} from './spa'

function run(): void {
  const teamName: string = core.getInput('team-name')
  const appName: string = core.getInput('app-name')
  const ingresses: string[] = core.getInput('ingress').split(',')
  const ingressClass: string = core.getInput('ingressClass')
  const environment: string = core.getInput('environment')
  const tenant: string = core.getInput('tenant')

  const err = validateInputs(
    teamName,
    appName,
    ingresses,
    ingressClass,
    environment,
    tenant
  )
  if (err) {
    core.setFailed(err.message)
    return
  }

  const {cdnDest, naisCluster, naisResources} = spaSetupTask(
    teamName,
    appName,
    ingresses,
    ingressClass,
    environment,
    tenant
  )

  const naisDeployServer = `deploy.${tenant}.cloud.nais.io:443`

  core.setOutput('cdn-destination', cdnDest)
  core.setOutput('nais-cluster', naisCluster)
  core.setOutput('nais-resource', naisResources)
  core.setOutput('nais-deploy-server', naisDeployServer)
}

run()
