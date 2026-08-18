// 设计令牌（与 artifact-registry 控制台统一的技术蓝风格，数值对齐其 theme/tokens.ts）。
import type { ThemeConfig } from 'antd'

export const brandColors = {
  primary: '#1677FF',
  primaryHover: '#4096FF',
  primaryActive: '#0958D9',
  primarySoft: '#E6F4FF',
  primarySofter: '#F0F7FF',
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
} as const

export const fontSize = {
  hero: 24,
  largeTitle: 20,
  h1: 18,
  list: 16,
  body: 14,
  secondary: 12,
  caption: 11,
} as const

export const layoutSize = {
  headerHeight: 56,
  siderWidth: 220,
  siderCollapsedWidth: 64,
  contentSafePadding: 24,
  pagePaddingBlock: 20,
  gutter: 24,
} as const

export const controlHeight = {
  large: 48,
  middle: 36,
  small: 28,
} as const

/** 与参考控制台一致的 antd 主题配置（技术蓝、浅色、克制动效）。 */
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
      '"PingFang SC", "Microsoft YaHei", "Source Han Sans SC", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    fontFamilyCode: 'ui-monospace, "SF Mono", Menlo, Monaco, Consolas, monospace',
    fontSize: fontSize.body,
    fontSizeHeading1: fontSize.hero,
    fontSizeHeading2: fontSize.largeTitle,
    fontSizeHeading3: fontSize.h1,
    fontSizeSM: fontSize.secondary,
    fontSizeLG: fontSize.list,
    lineHeight: 22 / 14,
    controlHeight: controlHeight.middle,
    controlHeightLG: controlHeight.large,
    controlHeightSM: controlHeight.small,
    borderRadius: 8,
    borderRadiusLG: 12,
    borderRadiusSM: 6,
    boxShadowTertiary: '0 2px 8px rgba(31, 35, 41, 0.04)',
    motion: false,
    motionDurationMid: '150ms',
  },
  components: {
    Layout: {
      headerHeight: layoutSize.headerHeight,
      headerBg: brandColors.primary,
      siderBg: neutralColors.bgContainer,
      bodyBg: neutralColors.bgLayout,
    },
    Menu: {
      itemHeight: 40,
      itemBorderRadius: 8,
      itemMarginBlock: 2,
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
      paddingInlineLG: 20,
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
      cellPaddingBlock: 12,
      cellPaddingInline: 16,
      headerColor: neutralColors.secondary,
    },
    Card: {
      paddingLG: 20,
      headerBg: 'transparent',
      colorBorderSecondary: neutralColors.border,
      borderRadiusLG: 12,
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
      activeShadow: '0 0 0 2px rgba(22, 119, 255, 0.12)',
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
