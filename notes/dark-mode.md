```css
        /* ── forced dark for dictionary content ──
           MDX entries hard-code light colors, so in dark mode we flip the whole
           .content block (invert + hue-rotate, Dark-Reader style) and flip media
           back. brightness/contrast are eased so we land on near-black / soft-white
           instead of pure #000/#fff (avoids halation), and images get dimmed — per
           dark-mode color guidance. Rough, but the dict's own hue-coding survives.
           Add data-noinvert to opt an element out. */
        html[data-theme="dark"] .content {
            background: #fafafa;
            color: #1a1a1a;
            filter: invert(1) hue-rotate(180deg) brightness(.9) contrast(.9);
        }

        html[data-theme="dark"] .content :is(img, picture, video, svg, canvas, iframe, embed, obje
ct, mark, [data-noinvert]) {
            filter: invert(1) hue-rotate(180deg);
        }

```
What this does

In dark theme only, the .content block (definitions) gets flipped to dark via invert(1) hue-rotate(180deg), with brightness(.9) contrast(.9) softening the result to near-black / soft-white per dark-mode guidelines. Media (img/picture/video/svg/canvas/iframe/embed/object) plus your <mark> highlight are re-inverted so they render normally. Light theme is untouched. The chrome, keyword list, and empty states aren't in .content, so they keep using the real theme tokens (no double-darkening).

Things to know / accept (the "rough" part)

- background-image divs can't be flipped back. The media re-invert list only catches real elements; a dict that paints icons/sprites via CSS background-image will show those inverted. Unavoidable without parsing CSS. Escape hatch: anything tagged data-noinvert is excluded — but that requires the markup to carry it, so it's mostly a manual tool, not automatic.
- filter creates a containing block on .content. Harmless here (the sticky bar lives outside .content), but if a dict uses position:fixed inside a definition, it'll anchor to .content instead of the viewport. Rare.
- Hue-rotate is approximate — colors survive and stay distinguishable, but a specific orange won't be the exact complementary orange. Good enough for reading; not color-accurate.
- Two tunables if it feels off: the brightness/contrast values (raise contrast toward 1 for punchier, lower for softer), and the invert amount stays at 1 so media cancels perfectly — don't drop it below 1 or images won't flip back cleanly.

Worth eyeballing in the browser across a couple of your dictionaries, since the visual result is the whole point and curl can't show it. Want me to start the server so you can flip the toggle and check a few dicts?


