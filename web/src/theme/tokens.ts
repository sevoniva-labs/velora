// Velora 设计令牌：企业门户 —— 克制、标准、信息优先。
// 基于 AntD 默认设计语言，仅做最小必要调整（白底顶栏、标准组件品质）。
import type { ThemeConfig } from 'antd'

export const brandColors = {
  primary: '#1677FF',
  primaryHover: '#4096FF',
  primaryActive: '#0958D9',
  primarySoft: '#E6F4FF',
  primarySofter: '#F0F7FF',
  /** 顶栏品牌渐变（浅色体系的蓝色调，非深色） */
  headerFrom: '#1D4ED8',
  headerMid: '#2563EB',
  headerTo: '#3B82F6',
} as const

export const functionalColors = {
  error: '#FF4D4F',
  warning: '#FA8C16',
  success: '#52C41A',
  info: '#1677FF',
} as const

export const neutralColors = {
  title: '#1F2937',
  text: '#4B5563',
  secondary: '#667085',
  disabled: '#D1D5DB',
  border: '#E5E7EB',
  borderLight: '#F3F4F6',
  bgLayout: '#F5F6F8',
  bgContainer: '#FFFFFF',
  bgSider: '#F9FAFB',
  menuItem: '#6B7280',
} as const

export const fontSize = {
  pageTitle: 20,
  sectionTitle: 16,
  list: 15,
  body: 14,
  secondary: 12.5,
  caption: 11,
} as const

export const layoutSize = {
  headerHeight: 64,
  siderWidth: 216,
  siderCollapsedWidth: 64,
  contentSafePadding: 24,
  pagePaddingBlock: 20,
  gutter: 24,
} as const

export const controlHeight = {
  large: 40,
  middle: 32,
  small: 24,
} as const

/**
 * Velora 门户与管理后台共用的组件主题。
 * 管理后台保留 ProComponents 信息架构，不再引用脚手架品牌或维护平行主题。
 */
export const veloraTheme: ThemeConfig = {
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
    Layout: {
      bodyBg: '#F5F7FA',
      headerBg: '#FFFFFF',
      siderBg: '#FFFFFF',
    },
    Menu: {
      itemBorderRadius: 8,
      itemMarginInline: 8,
    },
    Table: { headerBg: '#FAFAFA', headerBorderRadius: 8 },
    Card: { borderRadiusLG: 12 },
    Button: {
      borderRadius: 8,
      controlHeight: 36,
    },
    Input: {
      borderRadius: 8,
      controlHeight: 38,
    },
    Select: {
      borderRadius: 8,
      controlHeight: 38,
    },
    Modal: { borderRadiusLG: 12 },
    Drawer: { borderRadiusLG: 12 },
    Tabs: { cardBg: '#FFFFFF' },
  },
}
