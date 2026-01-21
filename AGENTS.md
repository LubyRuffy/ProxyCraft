# Agent Rules

## Frontend Change Protocol
- Always run `npm run build` in `web-react/` after any frontend change or dependency update.
- Do not report completion until the build passes with exit code 0.
- Avoid hardcoded color utility classes (e.g. `text-red-500`, `bg-amber-400`). Use theme token classes only (e.g. `text-primary`, `bg-accent`).
- Keep Tailwind v4 usage consistent with `@import "tailwindcss";` in `web-react/src/index.css`.
- Do not commit generated `.js` files under `web-react/src/`. Only `.ts/.tsx` source files should live there. If such files appear, delete them and ensure TypeScript is not emitting into `src/`.

## Scope Safety
- Operate only within this repository.
- Do not search or modify paths outside the repo root.
