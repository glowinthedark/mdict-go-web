The core structural problem (the big one)

Everything else is cosmetic next to this: the app's own UI and the dictionaries' own CSS share one global stylesheet and one DOM scope. Each .mdx ships its own style.css/inline styles, and you now hoist them into <head> (good for dedup) — but they still cascade over your search box, switcher, and layout, and your rules cascade into the definitions. That's the root of "unpolished": you can never make the chrome look intentional while arbitrary third-party CSS is fighting it. The single highest-leverage move is isolating dictionary content from app chrome (a sandboxed render region for definitions, your own clean styles everywhere else). Until that boundary exists, polishing the CSS is rearranging furniture in a room with no walls.

The rendering model

Right now every search and every internal link is a full server round-trip + whole-page reload. It works, but it feels like 2005 — no instant response, flash of white, scroll position lost, no sense of continuity. The backend already supports prefix matching, which you're not exposing interactively. A move to fetch-and-swap (results update in place) unlocks live search, suggestions, history/back behavior, and a "smooth" feel — and it's the prerequisite for most UX polish people actually notice.

The CSS itself is in disrepair

Concretely, not nitpicks: duplicate conflicting * rules (Georgia serif declared, then overridden to system sans), pre defined twice, .hl twice, the link block twice; invalid properties (z-order, background-color2); dead vendor prefixes (-moz/-o/-ms) everywhere; mixed units (pt/px/em/%) with no scale; magic colors scattered inline. There's no design-token layer — no single place that defines color, spacing, type scale. This is why nothing feels consistent: there's no system, just accreted rules.

Smaller but visible issues

- The dictionary name is 5pt grey on white — effectively invisible. Either commit to showing it or drop it.
- No states: no loading indicator, no styled empty/no-results state, no error state. Errors currently render as raw text.
- Highlight styling (greenyellow + dotted border) is garish and fights readability.
- Affordances are emoji (🔎, 📖, 🔊) with weak labels and low-contrast greys — fine as accents, weak as primary controls.
- Accessibility gaps: thin focus styles, low-contrast greys (#999/#aaa), no landmarks/aria, click-only audio.
- No dark mode — a reading-heavy tool is exactly where people want it.
- Mobile is one thin media query; the control row doesn't reflow gracefully.

How I'd sequence it (high-level, no code)

1