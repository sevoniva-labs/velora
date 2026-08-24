import { describe, expect, it } from 'vitest'
import { provisioningTargetFromResponse } from './api'

const target = {
  id: 'target-1',
  applicationId: 'app-1',
  endpointUrl: 'https://example.com/provisioning',
  deliveryStatus: 'HEALTHY',
  activeKeyVersion: '2',
  configVersion: '3',
}

describe('provisioningTargetFromResponse', () => {
  it('maps the flattened single-field response used by GET', () => {
    expect(provisioningTargetFromResponse(target)).toMatchObject({
      id: 'target-1',
      deliveryStatus: 'HEALTHY',
      activeKeyVersion: 2,
      configVersion: 3,
    })
  })

  it('maps the nested target used by mutation responses', () => {
    expect(provisioningTargetFromResponse({ target })).toMatchObject({
      id: 'target-1',
      endpointUrl: 'https://example.com/provisioning',
      deliveryStatus: 'HEALTHY',
    })
  })
})
