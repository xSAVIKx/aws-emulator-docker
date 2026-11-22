# CLAUDE.md

The project site is in `site/`. The Pages workflow (`.github/workflows/pages.yml`) publishes it.

## Design identity

This site uses the shared identity from xSAVIKx/xSAVIKx.github.io/identity.
- Do not edit the vendored `ys-*.css` file. To update it, copy the new file from the hub.
- Keep this project's own brand colours, logo and illustrations.
- Use system fonts only. Do not add a web font or serif headings.
- New text colours must meet 4.5:1 contrast in light and dark.

In this site, the vendored file is `site/ys-plain.css` (the plain adapter). The `:root` blocks in `site/index.html`
and `site/404.html` map `--bg`, `--bg-soft`, `--fg`, `--muted` and `--line` to the `--ys-*` neutrals.
The orange accent (`--accent`, `--accent-text`, `--accent-ink`) and the code band (`--code-bg`,
`--code-fg`) are the brand of this project. Keep them.
