import { describe, expect, it } from 'vitest'
import { veloraTheme } from './tokens'

describe('veloraTheme', () => {
  it('keeps the Ant Design theme aligned with the Forge scaffold', () => {
    expect(veloraTheme).toEqual({
      token: {
        colorPrimary: '#1677FF',
        colorInfo: '#1677FF',
        colorSuccess: '#16A34A',
        colorWarning: '#D97706',
        colorError: '#DC2626',
        colorBgLayout: '#F5F7FA',
        borderRadius: 8,
        borderRadiusLG: 12,
        controlHeight: 36,
        fontSize: 14,
      },
      components: {
        Layout: { bodyBg: '#F5F7FA', headerBg: '#FFFFFF', siderBg: '#FFFFFF' },
        Menu: { itemBorderRadius: 8, itemMarginInline: 8 },
        Table: { headerBg: '#FAFAFA', headerBorderRadius: 8 },
        Card: { borderRadiusLG: 12 },
        Button: { borderRadius: 8, controlHeight: 36 },
        Input: { borderRadius: 8, controlHeight: 38 },
        Select: { borderRadius: 8, controlHeight: 38 },
        Modal: { borderRadiusLG: 12 },
        Drawer: { borderRadiusLG: 12 },
        Tabs: { cardBg: '#FFFFFF' },
      },
    })
  })
})
