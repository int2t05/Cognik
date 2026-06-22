# OpsMind Design System

## Direction
Enterprise IT operations control panel. Calm, precise, quietly authoritative.
Status-driven visual language — every element communicates state.

## Depth Strategy
Borders-first with whisper-quiet hairline separators (`rgba(0,0,0,0.04)` light / `rgba(255,255,255,0.08)` dark).
Shadows reserved for overlays (dialog `0 20px 60px rgba(0,0,0,0.15)`, sidebar `2px 0 8px`).
Blur used only on sticky headers (backdrop-blur-xl) and dialog scrims — purposeful, not decorative.

## Spacing
- Base unit: 4px (Tailwind scale)
- Layout padding: p-5 (20px)
- Section gaps: mb-5 (20px) between major sections
- Component padding: 20px card, 12px table cell, 16px dialog content
- Icon gaps: gap-1 (4px) for tight, gap-2 (8px) for standard

## Type Scale (ratio ~1.25)
- hero 28px/1.15 — page titles
- headline 21px/1.2 — section headers
- title 17px/1.3 — card titles
- body 15px/1.47 — content text
- caption 13px/1.4 — labels, meta
- fine 12px — badges, footnotes
All headings: font-semibold. Body: font-regular. Hierarchy via weight + color, not size alone.

## Radius Scale
- sm: 8px — inputs, buttons, sidebar items
- md: 11px — cards (default)
- lg: 18px — message bubbles, modals
- pill: 9999px — filter pills, badges, pagination

## Color Tokens
### Brand
- accent: #0066cc (light) / #2997ff (dark)
- on-accent: #ffffff (both)

### Surfaces (light → dark), shift lightness only
- canvas: #ffffff → #1d1d1f
- parchment: #f5f5f7 → #161618
- pearl: #fafafc → #2a2a2c

### Text (4-level hierarchy)
- ink (primary): #1d1d1f → #f5f5f7
- muted-80 (secondary): #333333 → #cccccc
- muted-48 (tertiary): #7a7a7a → #666666
- on-accent (on brand): #ffffff (both)

### Borders
- hairline: #e0e0e0 → rgba(255,255,255,0.12)
- divider-soft: rgba(0,0,0,0.04) → rgba(255,255,255,0.08)

### Semantic
- success: #34c759, warning: #ff9500, error: #ff3b30, info: #007aff
- Each has bg/text badge variants for both themes

## Component Patterns
- `AppleButton` — 4 variants: pill (primary), ghost, utility, pearl. Base: py-1.5, px-4 pill / px-2.5 others, gap-1, transition-all duration-150, active:scale-95
- `AppleTable` — borderless, hairline bottom borders, hover highlight
- `AppleCard` — 20px padding, hairline border, radius-lg
- `AppleDialog` — Radix primitive, backdrop-blur scrim, shadow-dialog
- `ApplePagination` — rounded-full page buttons, icon prev/next
- `AppleInput` — 44px height, radius-sm, focus ring
- Filter pills — p-2, radius-pill, pearl bg default / accent bg active

## Icon Convention
- Admin: icon-only buttons (size 14-18px), aria-label for accessibility
- Portal: navigation keeps icon+text, action buttons icon-only
- Consistent Lucide icon set, outline style, 1.5px stroke

## States Required
Every interactive element: default, hover, active, focus-visible, disabled
Data views: loading, empty, error
Focus ring: `box-shadow: var(--focus-ring)` on :focus-visible
