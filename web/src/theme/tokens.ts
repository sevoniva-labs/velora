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
  secondary: '#9CA3AF',
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

/** Velora 主题：接近 AntD 默认，仅统一品牌色与少量布局。 */
export const veloraTheme: ThemeConfig = {
  cssVar: { prefix: 'ant' },
  token: {
    colorPrimary: brandColors.primary,
    colorPrimaryHover: brandColors.primaryHover,
    colorPrimaryActive: brandColors.primaryActive,
    colorInfo: brandColors.primary,
    colorSuccess: functionalColors.success,
    colorWarning: functionalColors.warning,
    colorError: functionalColors.error,
    colorText: neutralColors.title,
    colorTextSecondary: neutralColors.text,
    colorTextTertiary: neutralColors.secondary,
    colorTextQuaternary: neutralColors.disabled,
    colorBorder: neutralColors.border,
    colorBorderSecondary: neutralColors.borderLight,
    colorBgLayout: neutralColors.bgLayout,
    colorBgContainer: neutralColors.bgContainer,
    colorBgElevated: neutralColors.bgContainer,
    colorFillAlter: '#F7F8FA',
    colorLink: brandColors.primary,
    colorLinkHover: brandColors.primaryHover,
    colorPrimaryBg: brandColors.primarySoft,
    colorPrimaryBgHover: brandColors.primarySofter,
    fontFamily:
      '"PingFang SC", "Microsoft YaHei", "Source Han Sans SC", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    fontFamilyCode: 'ui-monospace, "SF Mono", Menlo, Monaco, Consolas, monospace',
    fontSize: fontSize.body,
    fontSizeHeading1: fontSize.pageTitle,
    fontSizeHeading2: fontSize.pageTitle,
    fontSizeHeading3: fontSize.sectionTitle,
    fontSizeHeading4: fontSize.list,
    fontSizeSM: fontSize.secondary,
    fontSizeLG: fontSize.list,
    lineHeight: 22 / 14,
    controlHeight: controlHeight.middle,
    controlHeightLG: controlHeight.large,
    controlHeightSM: controlHeight.small,
    borderRadius: 8,
    borderRadiusLG: 10,
    borderRadiusSM: 6,
    motion: false,
  },
  components: {
    Layout: {
      headerHeight: layoutSize.headerHeight,
      headerBg: brandColors.headerMid,
      siderBg: neutralColors.bgSider,
      bodyBg: neutralColors.bgLayout,
    },
    Menu: {
      itemHeight: 38,
      itemBorderRadius: 8,
      itemMarginBlock: 2,
      itemSelectedBg: brandColors.primarySoft,
      itemSelectedColor: brandColors.primary,
      itemHoverBg: brandColors.primarySofter,
      itemColor: neutralColors.menuItem,
      itemActiveBg: brandColors.primarySoft,
    },
    Button: {
      controlHeight: controlHeight.middle,
      controlHeightLG: controlHeight.large,
      controlHeightSM: controlHeight.small,
      paddingInline: 15,
      paddingInlineLG: 20,
      paddingInlineSM: 10,
      primaryShadow: 'none',
      defaultShadow: 'none',
      dangerShadow: 'none',
      fontWeight: 500,
    },
    Table: {
      headerBg: neutralColors.bgContainer,
      borderColor: neutralColors.borderLight,
      rowHoverBg: '#F7F9FC',
      cellPaddingBlock: 12,
      cellPaddingInline: 14,
      headerColor: neutralColors.text,
    },
    Card: {
      paddingLG: 20,
      headerBg: 'transparent',
      colorBorderSecondary: neutralColors.border,
      borderRadiusLG: 10,
    },
    Form: {
      labelFontSize: fontSize.body,
      itemMarginBottom: 20,
    },
    Input: {
      controlHeight: controlHeight.middle,
      controlHeightLG: controlHeight.large,
      controlHeightSM: controlHeight.small,
      activeBorderColor: brandColors.primary,
      hoverBorderColor: brandColors.primary,
      activeShadow: '0 0 0 2px rgba(22, 119, 255, 0.1)',
    },
    Tabs: {
      itemSelectedColor: brandColors.primary,
      inkBarColor: brandColors.primary,
      itemHoverColor: brandColors.primaryHover,
    },
    Tag: {
      defaultBg: '#F5F6F8',
      defaultColor: neutralColors.text,
    },
  },
}
