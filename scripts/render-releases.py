#!/usr/bin/env python3
"""Render the GitHub releases of this repository into the site.

Usage: render-releases.py <repo> <html-file>

It replaces the <!-- RELEASES --> marker in <html-file> with one
<details> block for each release. GitHub renders the release text from
Markdown to HTML through its /markdown API. Set GH_TOKEN for a higher
rate limit.
"""
import html
import json
import os
import sys
import urllib.request

API = "https://api.github.com"
MARKER = "<!-- RELEASES -->"
LIMIT = 20


def call(path, data=None):
    req = urllib.request.Request(API + path, data=data)
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    token = os.environ.get("GH_TOKEN")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    if data is not None:
        req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.read().decode("utf-8")


def markdown(text, repo):
    body = json.dumps({"text": text, "mode": "gfm", "context": repo}).encode()
    return call("/markdown", body)


def main():
    repo, page = sys.argv[1], sys.argv[2]
    releases = json.loads(call(f"/repos/{repo}/releases?per_page={LIMIT}"))
    releases = [r for r in releases if not r.get("draft")]
    parts = []
    if not releases:
        parts.append('<p class="muted">No releases yet.</p>')
    for i, r in enumerate(releases):
        date = (r.get("published_at") or r.get("created_at") or "")[:10]
        title = html.escape(r.get("name") or r["tag_name"])
        url = html.escape(r["html_url"])
        body = markdown(r.get("body") or "", repo)
        pill = '<span class="pill">latest</span>' if i == 0 else ""
        parts.append(
            f'<details class="release"{" open" if i == 0 else ""}>'
            f'<summary><b>{title}</b><span class="muted">{date}</span>'
            f"{pill}</summary>"
            f'<div class="release-body">{body}'
            f'<p><a href="{url}">View on GitHub</a></p></div></details>'
        )
    with open(page, encoding="utf-8") as f:
        content = f.read()
    if MARKER not in content:
        sys.exit(f"{MARKER} not found in {page}")
    with open(page, "w", encoding="utf-8") as f:
        f.write(content.replace(MARKER, "\n".join(parts)))
    print(f"Rendered {len(releases)} releases")


if __name__ == "__main__":
    main()
