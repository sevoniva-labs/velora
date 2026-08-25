# Admin toolbar list visual QA

- Source visual truth: `/var/folders/jb/m4t41pjn3nzd5pg0qsr9n02c0000gn/T/codex-clipboard-dbe3cc3b-3050-4b00-8aad-0733335c6595.png`
- Implementation capture: `/Users/chuncheng/Downloads/code/velora/design-qa-implementation-component.png`
- Combined comparison: `/Users/chuncheng/Downloads/code/velora/design-qa-comparison.png`
- Browser viewport: 1391 × 756 CSS px, device scale 1
- Source pixels: 1670 × 790; implementation component pixels: 1123 × 328
- Normalization: both images were placed in one browser-rendered comparison at the same available content width; comparison focused on the list surface rather than the surrounding admin shell.
- State: desktop, light theme, populated application list, first scope selected.

## Full-view comparison evidence

The implementation matches the source structure: one list surface, scope filters with count badges on the left, persistent search and primary create action on the right, rows beneath the toolbar, and lifecycle actions aligned to the right. It retains the existing Velora portal typography, colors, density and shell tokens instead of copying the reference application's branding.

## Focused comparison evidence

The list card was captured separately so toolbar alignment, count badges, search button treatment, row spacing, action placement and bottom pagination could be compared without browser chrome or sidebar differences. Search and create controls share a 36 px control height; the search action uses the source's neutral outlined treatment. Pagination remains at the list bottom with stable spacing.

## Required fidelity surfaces

- Typography: existing Velora font stack and hierarchy retained; active scope, row title and secondary copy remain distinct.
- Spacing: toolbar and row padding are consistent; no detached filter card or floating filter popover remains.
- Color: existing portal tokens are used; no new gradient or unrelated palette was introduced.
- Assets: application icons use the existing `AppIcon` component; no placeholder SVG or CSS-drawn asset was added.
- Copy: controls use concise product language: scope names, search target and primary action only.

## Comparison history

1. P1: the previous implementation used dropdown filter triggers instead of a persistent toolbar. Fixed by introducing the shared ProComponents list toolbar with visible scopes and search.
2. P2: the first pass used a blue search button and inconsistent search/create heights. Fixed with a neutral outlined search action and unified 36 px controls.
3. Post-fix evidence: the combined comparison shows no remaining actionable P0, P1 or P2 mismatch for the requested list-toolbar pattern.

## Browser checks

- Primary application list rendered with realistic data.
- Scope tabs, persistent search, create action, row lifecycle actions and pagination were present in the accessibility snapshot.
- No legacy `.ant-pro-table-search` surface remained on the reviewed admin list pages.
- No horizontal viewport overflow was observed.
- All 23 admin `ProList`/`ProTable` instances now declare pagination; primary short-list measurements placed the pagination footer 25 px above the card bottom on applications, users and user groups.

final result: passed
