import YAML from 'yaml'
import {mkdirSync, writeFileSync} from 'fs'
import {ingressesForApp, serviceForApp} from './k8s'

type Clusters = {
  [key: string]: NaisCluster
}

export type Ingress = {
  ingressHost: string
  ingressPath: string
  ingressClass: string
}

const tenants = ['nav', 'ssb', 'test-nais']

const hostMap: {[index: string]: Clusters} = {
  nav: {
    'nav.no': {
      naisCluster: 'prod-gcp',
      ingressClass: 'nais-ingress-external'
    },
    'intern.nav.no': {
      naisCluster: 'prod-gcp',
      ingressClass: 'nais-ingress'
    },
    'ansatt.nav.no': {
      naisCluster: 'prod-gcp',
      ingressClass: 'nais-ingress-fa'
    },
    'dev.nav.no': {
      naisCluster: 'dev-gcp',
      ingressClass: 'nais-ingress-external'
    },
    'dev.intern.nav.no': {
      naisCluster: 'dev-gcp',
      ingressClass: 'nais-ingress'
    },
    'intern.dev.nav.no': {
      naisCluster: 'dev-gcp',
      ingressClass: 'nais-ingress'
    },
    'ansatt.dev.nav.no': {
      naisCluster: 'dev-gcp',
      ingressClass: 'nais-ingress-fa'
    },
    'ekstern.dev.nav.no': {
      naisCluster: 'dev-gcp',
      ingressClass: 'nais-ingress-external'
    }
  },
  ssb: {
    'test.ssb.cloud.nais.io': {
      naisCluster: 'test',
      ingressClass: 'nais-ingress'
    },
    'external.test.ssb.cloud.nais.io': {
      naisCluster: 'test',
      ingressClass: 'nais-ingress-external'
    },
    'intern.test.ssb.no': {
      naisCluster: 'test',
      ingressClass: 'nais-ingress'
    },
    'test.ssb.no': {
      naisCluster: 'test',
      ingressClass: 'nais-ingress-external'
    },
    'prod.ssb.cloud.nais.io': {
      naisCluster: 'prod',
      ingressClass: 'nais-ingress'
    },
    'external.prod.ssb.cloud.nais.io': {
      naisCluster: 'prod',
      ingressClass: 'nais-ingress-external'
    },
    'ssb.no': {
      naisCluster: 'prod',
      ingressClass: 'nais-ingress'
    },
    'intern.ssb.no': {
      naisCluster: 'prod',
      ingressClass: 'nais-ingress'
    }
  },
  'test-nais': {
    'sandbox.test-nais.cloud.nais.io': {
      naisCluster: 'sandbox',
      ingressClass: 'nais-ingress'
    },
    'external.sandbox.test-nais.cloud.nais.io': {
      naisCluster: 'sandbox',
      ingressClass: 'nais-ingress-external'
    }
  }
}

type NaisCluster = {
  naisCluster: string
  ingressClass: string
}

export function splitFirst(s: string, sep: string): [string, string] {
  const [first, ...rest] = s.split(sep)
  return [first, rest.join(sep)]
}

export function domainForHost(host: string): string {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_, domain] = splitFirst(host, '.')

  return domain
}

export function isValidIngress(tenant: string, ingresses: string[]): boolean {
  for (const ingress of ingresses) {
    try {
      const url = new URL(ingress)
      if (parseIngress(tenant, url.host) === undefined) {
        return false
      }
    } catch {
      return false
    }
  }

  return true
}

export function isValidAppName(app: string): boolean {
  // RFC 1123 https://tools.ietf.org/html/rfc1123#section-2
  return /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(app)
}

export function parseIngress(tenant: string, ingressHost: string): NaisCluster {
  return hostMap[tenant][domainForHost(ingressHost)]
}

export function cdnPathForApp(team: string, app: string, env: string): string {
  return `/${team}/${cdnDestForApp(app, env)}`
}

export function cdnBucketVhost(tenant: string): string {
  return `cdn.${tenant}.cloud.nais.io`
}

export function cdnDestForApp(app: string, env: string): string {
  return `${app}/${env}`
}

export function naisResourcesForApp(
  team: string,
  app: string,
  env: string,
  ingresses: Ingress[],
  bucketPath: string,
  bucketVhost: string,
  tmpDir = './tmp'
): string {
  const filePaths: string[] = []
  const serviceResource = serviceForApp(team, app, env, bucketVhost)
  const ingressesResource = ingressesForApp(
    team,
    app,
    env,
    ingresses,
    bucketPath,
    bucketVhost
  )

  mkdirSync(tmpDir, {recursive: true})

  for (const item of [serviceResource, ...ingressesResource.items]) {
    const name = item.metadata?.name || 'unknown'
    const type = item.kind?.toLowerCase() || 'unknown'
    const path = `${tmpDir}/${name}-${type}.yaml`

    filePaths.push(path)
    writeFileSync(path, YAML.stringify(item))
  }

  return filePaths.join(',')
}

export function validateInputs(
  team: string,
  app: string,
  ingress: string[],
  ingressClass: string,
  environment: string,
  tenant: string
): Error | null {
  if (!isValidAppName(team)) {
    return Error(
      `SPADEPLOY-001: Invalid team name: ${team}. Team name must match regex: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    )
  }

  if (!isValidAppName(app)) {
    return Error(
      `SPADEPLOY-002: Invalid app name: ${app}. App name must match regex: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    )
  }

  if (!isValidAppName(environment)) {
    return Error(
      `SPADEPLOY-003: Invalid environment name: ${environment}. Environment name must match regex: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    )
  }

  if (ingress.length === 0) {
    return Error('SPADEPLOY-004: No ingress specified')
  }

  if (hasCustomIngressClass(ingressClass)) {
    return null
  }

  if (!tenants.includes(tenant)) {
    return Error(
      `SPADEPLOY-007: Invalid tenant name: ${tenant}. Tenant name must be added to the list of valid tenants`
    )
  }

  if (!isValidIngress(tenant, ingress)) {
    return Error(
      `SPADEPLOY-006: Invalid ingress: ${ingress}. Ingress must be a valid URL with a known domain on format https://<host>/<path>`
    )
  }

  return null
}

export function hasCustomIngressClass(ingressClass: string): boolean {
  return ingressClass !== ''
}

export function spaSetupTask(
  team: string,
  app: string,
  urls: string[],
  customIngressClass: string,
  env = '',
  tenant: string
): {
  cdnDest: string
  naisCluster: string
  naisResources: string
} {
  let naisClusterFinal = ''

  const ingresses: Ingress[] = []

  if (hasCustomIngressClass(customIngressClass)) {
    const {hostname: ingressHost, pathname: ingressPath} = new URL(urls[0])
    ingresses.push({ingressHost, ingressPath, ingressClass: customIngressClass})
    naisClusterFinal = env
  } else {
    for (const ingress of urls) {
      const {hostname: ingressHost, pathname: ingressPath} = new URL(ingress)
      const {naisCluster, ingressClass} = parseIngress(tenant, ingressHost)

      ingresses.push({ingressHost, ingressPath, ingressClass})

      naisClusterFinal = naisClusterFinal || naisCluster

      if (naisClusterFinal !== naisCluster) {
        throw Error(
          `SPADEPLOY-005: Ingresses must be on same cluster. Found ${naisClusterFinal} and ${naisCluster}`
        )
      }
    }
  }

  env = env || naisClusterFinal
  const bucketPath = cdnPathForApp(team, app, env)
  const bucketVhost = cdnBucketVhost(tenant)
  const cdnDest = cdnDestForApp(app, env)
  const naisResources = naisResourcesForApp(
    team,
    app,
    env,
    ingresses,
    bucketPath,
    bucketVhost
  )

  return {
    cdnDest,
    naisCluster: naisClusterFinal,
    naisResources
  }
}
