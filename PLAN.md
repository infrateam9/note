# UI Redesign Plan — White & Blue Modern Theme

## Current State

- Basic functional UI: header bar, full-page textarea, status bar
- Flat gray (#f5f5f5) background, white header, generic blue (#007bff) buttons
- **Mobile broken**: buttons overflow horizontally, no responsive layout
- No visual hierarchy, no polish, no transitions
- Looks like a 2015 Bootstrap prototype

## Design Goals

1. **Modern & clean** — white-dominant with blue accents, subtle shadows, rounded corners
2. **Responsive** — first-class mobile support (stacked layout, hamburger or icon buttons)
3. **Focused writing experience** — textarea is the hero, minimal chrome
4. **Consistent blue palette** — primary `#2563EB`, hover `#1D4ED8`, light tint `#EFF6FF`
5. **Smooth interactions** — transitions on hover, save status indicator, focus ring

## Color Palette

| Role | Color | Usage |
|---|---|---|
| Primary | `#2563EB` | Buttons, links, active states |
| Primary Hover | `#1D4ED8` | Button hover |
| Primary Light | `#EFF6FF` | Status bar bg, subtle highlights |
| Background | `#FFFFFF` | Page background |
| Surface | `#F8FAFC` | Textarea background |
| Border | `#E2E8F0` | Dividers, textarea border |
| Text Primary | `#1E293B` | Headings |
| Text Secondary | `#64748B` | Note ID, status text |
| Text Muted | `#94A3B8` | Placeholder |

## Layout Changes

### Desktop (≥768px)
```
┌──────────────────────────────────────────────┐
│  📝 Note  ·  note-id        [actions row]    │  ← slim header, single line
├──────────────────────────────────────────────┤
│                                              │
│   textarea (full width, slight inset)        │
│                                              │
│                                              │
├──────────────────────────────────────────────┤
│  ● Ready                              chars  │  ← status bar
└──────────────────────────────────────────────┘
```

### Mobile (<768px)
```
┌────────────────────────┐
│  📝 Note    [≡] menu   │  ← compact header
├────────────────────────┤
│                        │
│   textarea (full)      │
│                        │
│                        │
├────────────────────────┤
│  ● Ready               │
└────────────────────────┘
```

Mobile actions: icon-only buttons in a row below the title, or a dropdown menu.

## Specific Changes (all in `renderHTML` in `handlers.go`)

### Phase 1 — Core Visual Refresh ✅
- [x] Update color scheme to white + blue palette above
- [x] Add `box-shadow` to header (subtle elevation)
- [x] Rounded corners on textarea with light border
- [x] Better typography: slightly larger title, inter-font stack
- [x] Button redesign: outlined with icons, hover transition
- [x] Status bar: dot indicator (green=saved, blue=saving, gray=ready, red=error)
- [x] Smooth CSS transitions on interactive elements
- [x] Toast notifications for copy actions

### Phase 2 — Responsive / Mobile ✅
- [x] Add `@media (max-width: 640px)` breakpoints
- [x] Icon-only buttons on mobile (labels hidden)
- [x] Reduce padding on mobile for more writing space
- [x] Hide note ID badge on mobile to save space
- [x] Ensure textarea fills available height (`flex: 1`, `100dvh`)
- [x] Tested on 375px (iPhone SE) and 390px (iPhone 14) widths

### Phase 3 — Polish ✅
- [x] Add character/word count in status bar (right side)
- [x] Focus ring on textarea (blue glow with `box-shadow`)
- [x] Animate status text transitions
- [x] Print styles cleanup (0.5in padding, proper font size)
- [x] Blue pencil icon accent in header
- [x] CSS custom properties for consistent theming

## Non-Goals (keep it simple)

- No dark mode (for now)
- No JavaScript frameworks
- No external CSS libraries
- No separate static files — everything stays inline in `renderHTML`
- No changes to Go backend logic, storage, or API

## Testing

1. `go test -v ./...` — ensure no regressions
2. Visual test on desktop (1280×720) and mobile (375×667)
3. Verify auto-save still works
4. Verify all buttons (New Note, Copy Content, Copy Link, Print) work
5. Verify curl interface unchanged
