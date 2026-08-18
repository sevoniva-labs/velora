// Velora 设计令牌：企业门户视觉系统（沉稳企业蓝 + 深海军蓝深色面）。
import type { ThemeConfig } from 'antd'

export const brandColors = {
  primary: '#2563EB',
  primaryHover: '#3B82F6',
  primaryActive: '#1D4ED8',
  primarySoft: '#EAF1FE',
  primarySofter: '#F4F8FF',
  /** 深色面（顶栏 / Hero）：深海军蓝黑 */
  inkDeep: '#0B1626',
  inkMid: '#10253F',
  inkLight: '#163A66',
  inkGlow: '#1F5BB5',
} as const

export const functionalColors = {
  error: '#E5484D',
  warning: '#F5A524',
  success: '#2F9E63',
  info: '#2563EB',
} as const

export const neutralColors = {
  title: '#0F172A',
  text: '#3E4C5F',
  secondary: '#8494A7',
  faint: '#A9B5C5',
  disabled: '#CBD5E1',
  border: '#E3EAF4',
  borderLight: '#EEF3FA',
  bgLayout: '#F3F6FB',
  bgSider: '#F6F8FC',
  bgContainer: '#FFFFFF',
  bgElevated: '#F7F9FD',
  menuItem: '#5C6B80',
  onInk: '#FFFFFF',
  onInkMuted: 'rgba(255, 255, 255, 0.72)',
} as const

export const fontSize = {
  display: 30,
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

/** Velora 主题：企业门户风格（沉稳蓝、深海军顶栏、克制动效）。 */
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
    boxShadowTertiary: '0 1px 2px rgba(15, 23, 42, 0.04), 0 8px 24px -12px rgba(15, 23, 42, 0.08)',
    motion: false,
    motionDurationMid: '180ms',
  },
  components: {
    Layout: {
      headerHeight: layoutSize.headerHeight,
      headerBg: brandColors.inkDeep,
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
      activeShadow: '0 0 0 3px rgba(37, 99, 235, 0.12)',
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
