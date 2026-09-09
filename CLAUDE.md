# hexplore

Read-only Bitcoin block explorer TUI. Go + Bubble Tea.
Full plan: @docs/PLAN.md

## Conventions
- Dependency rule: ui → domain ← provider. UI never imports a provider
  package and never sees provider JSON field names.
- Providers declare Capabilities. Unsupported features return ErrUnsupported
  and render as "—" with a reason. Never fabricate a value.
- Derived/approximate values are prefixed "~" in the UI.
- No wallet, no keys, no signing. Read-only by design.
