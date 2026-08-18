// Velora 设计令牌：企业门户视觉系统（浅色技术蓝，与原版一致；仅保留布局/质感层次改进）。
import type { ThemeConfig } from 'antd'

export const brandColors = {
  primary: '#1677FF',
  primaryHover: '#4096FF',
  primaryActive: '#0958D9',
  primarySoft: '#E6F4FF',
  primarySofter: '#F0F7FF',
  /** 顶栏 / Hero 品牌渐变（浅色体系） */
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
  text: '#475467',
  secondary: '#98A2B3',
  disabled: '#D0D5DD',
  border: '#E8EEF7',
  borderLight: '#F2F4F7',
  bgLayout: '#F5F7FA',
  bgSider: '#EDF1F8',
  bgContainer: '#FFFFFF',
  bgElevated: '#F5F7FA',
  menuItem: '#667085',
  onPrimary: '#FFFFFF',
} as const

export const fontSize = {
  display: 28,
  hero: 22,
  largeTitle: 18,
  h1: 16,
  list: 15,
  body: 14,
  secondary: 12.5,
  caption: 11,
} as const

export const layoutSize = {
  headerHeight: 64,
  siderWidth: 232,
  siderCollapsedWidth: 64,
  contentSafePadding: 24,
  pagePaddingBlock: 24,
  gutter: 24,
} as const

export const controlHeight = {
  large: 44,
  middle: 34,
  small: 28,
} as const

/** Velora 主题：浅色技术蓝企业门户（克制动效）。 */
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
    colorFillAlter: neutralColors.bgElevated,
    colorLink: brandColors.primary,
    colorLinkHover: brandColors.primaryHover,
    colorPrimaryBg: brandColors.primarySoft,
    colorPrimaryBgHover: brandColors.primarySofter,
    fontFamily:
      'Inter, "PingFang SC", "Microsoft YaHei", "Source Han Sans SC", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    fontFamilyCode: 'ui-monospace, "SF Mono", Menlo, Monaco, Consolas, monospace',
    fontSize: fontSize.body,
    fontSizeHeading1: fontSize.display,
    fontSizeHeading2: fontSize.hero,
    fontSizeHeading3: fontSize.largeTitle,
    fontSizeHeading4: fontSize.h1,
    fontSizeSM: fontSize.secondary,
    fontSizeLG: fontSize.list,
    lineHeight: 22 / 14,
    controlHeight: controlHeight.middle,
    controlHeightLG: controlHeight.large,
    controlHeightSM: controlHeight.small,
    borderRadius: 10,
    borderRadiusLG: 16,
    borderRadiusSM: 8,
    boxShadowTertiary: '0 1px 2px rgba(31, 41, 55, 0.04), 0 8px 24px -12px rgba(31, 41, 55, 0.08)',
    motion: false,
    motionDurationMid: '180ms',
  },
  components: {
    Layout: {
      headerHeight: layoutSize.headerHeight,
      headerBg: brandColors.primary,
      siderBg: neutralColors.bgSider,
      bodyBg: neutralColors.bgLayout,
    },
    Menu: {
      itemHeight: 40,
      itemBorderRadius: 10,
      itemMarginBlock: 3,
      itemSelectedBg: brandColors.primarySoft,
      itemSelectedColor: brandColors.primary,
      itemHoverBg: brandColors.primarySofter,
      itemColor: neutralColors.menuItem,
      itemActiveBg: brandColors.primarySoft,
      subMenuItemBg: 'transparent',
    },
    Button: {
      controlHeight: controlHeight.middle,
      controlHeightLG: controlHeight.large,
      controlHeightSM: controlHeight.small,
      paddingInline: 16,
      paddingInlineLG: 22,
      paddingInlineSM: 12,
      primaryShadow: 'none',
      defaultShadow: 'none',
      dangerShadow: 'none',
      fontWeight: 500,
    },
    Table: {
      headerBg: neutralColors.bgContainer,
      borderColor: neutralColors.borderLight,
      rowHoverBg: brandColors.primarySofter,
      cellPaddingBlock: 13,
      cellPaddingInline: 16,
      headerColor: neutralColors.secondary,
    },
    Card: {
      paddingLG: 22,
      headerBg: 'transparent',
      colorBorderSecondary: neutralColors.border,
      borderRadiusLG: 16,
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
      activeShadow: '0 0 0 3px rgba(22, 119, 255, 0.12)',
    },
    Tabs: {
      itemSelectedColor: brandColors.primary,
      inkBarColor: brandColors.primary,
      itemHoverColor: brandColors.primaryHover,
    },
    Tag: {
      defaultBg: neutralColors.bgElevated,
      defaultColor: neutralColors.text,
    },
  },
}
