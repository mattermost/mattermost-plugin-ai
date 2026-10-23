# webapp/AGENTS.md

Scoped instructions for the React/TypeScript webapp in `webapp/`. Root rules in `/AGENTS.md` still apply; only deviations and package-specific gotchas live here.

- Async loaders ignore responses from superseded requests (track the latest request, or cancel on input change and unmount) so a stale response never overwrites newer state.
- Use `@mattermost/compass-ui` components (buttons, icon buttons, tabs, radio groups, modals, menus) instead of hand-rolled interactive elements; anything clickable is keyboard-operable and has an accessible label.
- The `agents/*` lint rules are jsx-a11y rules that resolve styled-components declared in the same file to their DOM tag. Styled components imported from another file are invisible to them, so prefer native elements (`styled.button`) over adding handlers to imported wrappers.
