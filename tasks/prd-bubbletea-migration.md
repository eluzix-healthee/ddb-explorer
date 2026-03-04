# PRD: Bubble Tea Migration for DDB Explorer

## Introduction

Migrate the DDB Explorer terminal UI from `tview`/`tcell` to the Charm stack (`bubbletea`, `bubbles`, `lipgloss`) while preserving existing behavior and keyboard workflows. The migration should keep AWS access logic unchanged and produce a cleaner, modular TUI architecture that is easier to maintain and improve. A first-class requirement is consistent layout quality: all screens and modal-like views must be centered within the available content area.

## Goals

- Deliver full feature parity with the current TUI behavior.
- Move UI rendering and input handling to Bubble Tea architecture.
- Ensure all primary screens are centered and visually consistent.
- Preserve and improve keyboard-first navigation and shortcuts.
- Keep AWS client behavior unchanged.

## User Stories

### US-001: Add Bubble Tea Dependencies
**Description:** As a developer, I want Bubble Tea stack dependencies configured so the new TUI can be built and run.

**Acceptance Criteria:**
- [ ] `go.mod` includes `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, and `github.com/charmbracelet/lipgloss`.
- [ ] `go mod tidy` completes without errors.
- [ ] Existing non-TUI packages still compile.

### US-002: Create TUI Package Skeleton
**Description:** As a developer, I want a modular `tui/` package structure so view logic is decomposed and maintainable.

**Acceptance Criteria:**
- [ ] `tui/` contains separate files for app model, components, views, styles, keys, and messages.
- [ ] `main.go` is reduced to startup wiring and app launch.
- [ ] Package compiles with no circular imports.

### US-003: Define Shared Styles and Color Palette
**Description:** As a developer, I want centralized style definitions so all screens use consistent visual primitives.

**Acceptance Criteria:**
- [ ] Shared colors and `lipgloss.Style` values are defined in one place.
- [ ] Title, hint, error, success, and highlight styles are available for all views.
- [ ] Help/status chrome style matches the rest of the app.

### US-004: Centralize Key Bindings and View Messages
**Description:** As a developer, I want key maps and typed messages centralized so input behavior is consistent and testable.

**Acceptance Criteria:**
- [ ] Global keys (quit, back, help) are defined once and reused.
- [ ] View-specific keys (table navigation, query form focus, results actions) are explicitly grouped.
- [ ] Custom `tea.Msg` types are defined for load/query/scan success and failure paths.

### US-005: Implement Loading Screen
**Description:** As a user, I want a responsive loading screen with progress feedback so I know the app is actively connecting.

**Acceptance Criteria:**
- [ ] Loading screen shows spinner and context text.
- [ ] Loading content is horizontally and vertically centered.
- [ ] Window resize keeps loading view centered.

### US-006: Implement Table List Screen
**Description:** As a user, I want to browse and filter DynamoDB tables so I can quickly choose a target table.

**Acceptance Criteria:**
- [ ] Screen shows table name, item count, size, and status columns.
- [ ] Live filter input narrows visible table rows.
- [ ] Selected row can be opened for query/scan flow.
- [ ] Main content area remains centered within the chrome container.

### US-007: Implement Query and Scan Forms
**Description:** As a user, I want to enter query parameters or run a scan so I can fetch table items efficiently.

**Acceptance Criteria:**
- [ ] Query form supports partition key, optional sort key, and optional index selection.
- [ ] Scan mode allows full-table scan with explicit action.
- [ ] Field focus order and keyboard controls are predictable.
- [ ] Form containers are centered and stable on terminal resize.

### US-008: Implement Results View with Pagination
**Description:** As a user, I want paginated results with keyboard navigation so I can inspect items quickly.

**Acceptance Criteria:**
- [ ] Results table columns adapt to item schema.
- [ ] Pagination state (page, total, selection) is visible.
- [ ] Navigation keys support moving rows and changing pages.
- [ ] Results screen content is centered and does not overlap help/status bars.

### US-009: Implement Item Detail and JSON Modal
**Description:** As a user, I want to inspect a selected item in detail and open raw JSON when needed.

**Acceptance Criteria:**
- [ ] Item detail view renders key-value pairs in a readable table.
- [ ] JSON modal supports scrolling and close/back actions.
- [ ] Modal appears centered relative to terminal dimensions.
- [ ] Nested/large values remain readable without breaking layout.

### US-010: Integrate App State Machine
**Description:** As a developer, I want one Bubble Tea app model controlling view transitions so behavior is predictable.

**Acceptance Criteria:**
- [ ] App model handles all view transitions explicitly through view-state enum values.
- [ ] `tea.WindowSizeMsg` updates dimensions and propagates to active components.
- [ ] Shared chrome (help + status) wraps all views consistently.
- [ ] No transition leaves the app in an invalid partial state.

### US-011: Wire Async Data Operations and Error States
**Description:** As a user, I want robust loading and error feedback so failures are clear and recoverable.

**Acceptance Criteria:**
- [ ] Table loading, querying, and scanning run through Bubble Tea commands.
- [ ] User-facing errors are displayed in the status area or view context.
- [ ] Recoverable failures do not crash the app.
- [ ] Internal invariant violations are surfaced as explicit failures during development.

### US-012: Add Tests and Migration Verification
**Description:** As a developer, I want regression coverage and a parity checklist so migration confidence is high.

**Acceptance Criteria:**
- [ ] Unit tests cover core view-state transitions and key update paths.
- [ ] Tests cover resize behavior and centering helper/layout functions.
- [ ] Manual parity checklist confirms old functionality is available in new TUI.
- [ ] `go test ./...` passes.

## Functional Requirements

1. FR-1: The system must replace `tview`/`tcell` rendering and input flow with Bubble Tea.
2. FR-2: The system must keep AWS data-access APIs unchanged and reuse existing client behavior.
3. FR-3: The system must provide a centralized app model with explicit view-state transitions.
4. FR-4: The system must render a shared help bar and status bar across all primary views.
5. FR-5: The system must keep all primary screens centered in the available content region.
6. FR-6: The system must keep modal overlays centered relative to terminal width and height.
7. FR-7: The system must handle terminal resize events and recalculate layout deterministically.
8. FR-8: The system must preserve keyboard-driven workflows for navigation and actions.
9. FR-9: The system must provide clear error feedback for external failures.
10. FR-10: The system must expose loading states for table fetch, query, and scan operations.

## Non-Goals (Out of Scope)

- Rewriting or optimizing AWS DynamoDB client behavior.
- Adding brand-new data features not present in the current app.
- Implementing mouse-first interactions.
- Adding network retries beyond current behavior semantics.
- Introducing external theme engines or plugin systems.

## Design Considerations

- Keep information density suitable for terminal use on common widths.
- Prefer clear spacing and alignment over decorative styling.
- Use one centering strategy in the app layout wrapper to avoid per-screen drift.
- Keep selected-row and focused-input styling highly legible.

## Technical Considerations

- Keep resource ownership explicit in Bubble Tea commands and updates.
- Ensure all recoverable command errors are returned as messages and rendered.
- Avoid unbounded loops and implicit background work.
- Keep layout helpers pure where possible to simplify tests.

## Success Metrics

- 100% of baseline flows (table list, query/scan, results, detail, JSON view) work in Bubble Tea.
- All primary screens remain centered after terminal resize in manual verification.
- Keyboard-only usage can complete end-to-end data inspection without regressions.
- Test suite passes with added migration regression tests.

## Open Questions

- Should help text be condensed dynamically on very small terminal widths?
- Should centered layout use fixed max content width or full-width centering only?
- Should JSON modal include in-modal search in initial migration scope or follow-up?
