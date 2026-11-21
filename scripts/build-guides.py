#!/usr/bin/env python3
"""Build one guide page for each language in site/guides/<lang>/index.html.

Usage: build-guides.py <repo> <site-dir>

A guide page has the example README (examples/<lang>/README.md) and, when it
exists, the Testcontainers module README (testcontainers/<lang>/README.md).
GitHub renders the Markdown through its /markdown API. The page uses the
style and the script of site/index.html. The script also adds the guides to
site/sitemap.xml. Set GH_TOKEN for a higher API rate limit.
"""
import html
import json
import os
import re
import sys
import urllib.parse
import urllib.request

SITE = "https://xsavikx.github.io/aws-emulator-docker/"
BLOB = "https://github.com/xSAVIKx/aws-emulator-docker/blob/main/"
TREE = "https://github.com/xSAVIKx/aws-emulator-docker/tree/main/"

# slug, display name, test framework, SDK, Testcontainers library name
LANGUAGES = [
    ("python", "Python", "pytest", "Boto3", "testcontainers-python"),
    ("typescript", "TypeScript", "Vitest", "the AWS SDK for JavaScript v3", "Testcontainers for Node.js"),
    ("java", "Java", "JUnit 5", "the AWS SDK for Java 2.x", "Testcontainers for Java"),
    ("go", "Go", "go test", "the AWS SDK for Go v2", "Testcontainers for Go"),
    ("dotnet", "C# and .NET", "xUnit", "the AWS SDK for .NET", "Testcontainers for .NET"),
    ("rust", "Rust", "cargo test", "the AWS SDK for Rust", "testcontainers-rs"),
]


def markdown(text, repo):
    body = json.dumps({"text": text, "mode": "gfm", "context": repo}).encode()
    req = urllib.request.Request("https://api.github.com/markdown", data=body)
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("Content-Type", "application/json")
    token = os.environ.get("GH_TOKEN")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.read().decode("utf-8")


def absolute_links(text, base):
    """Make relative Markdown link targets absolute GitHub links."""

    def fix(match):
        target = match.group(1)
        if re.match(r"^(https?:|mailto:|#)", target):
            return match.group(0)
        url = urllib.parse.urljoin(base, target)
        if url.endswith("/"):
            url = url.replace("/blob/main/", "/tree/main/", 1)
        return f"]({url})"

    return re.sub(r"\]\(([^)\s]+)\)", fix, text)


def demote(text):
    """Move each heading one level down (h1 becomes h2), outside code blocks."""
    out, in_code = [], False
    for line in text.splitlines():
        if line.startswith("```"):
            in_code = not in_code
        if not in_code and re.match(r"^#{1,5} ", line):
            line = "#" + line
        out.append(line)
    return "\n".join(out)


def section(path, repo, root):
    text = open(os.path.join(root, path), encoding="utf-8").read()
    base = BLOB + os.path.dirname(path) + "/"
    rendered = markdown(demote(absolute_links(text, base)), repo)
    # GitHub adds anchors with a "user-content-" prefix; keep plain ids.
    rendered = rendered.replace('id="user-content-', 'id="')
    return rendered


def page(lang, name, framework, sdk, tc_name, body, has_tc, style, script, theme_svg, github_svg):
    title = f"Test AWS code in {name} with the AWS Emulator"
    desc = (f"Run {name} integration tests against local AWS APIs with the AWS Emulator Docker image: "
            f"{framework} and {sdk}" + (f", or {tc_name} with the AWS Emulator module" if has_tc else "")
            + ". STS, S3, DynamoDB, SQS, SNS and Secrets Manager without an AWS account.")
    url = f"{SITE}guides/{lang}/"
    ld = {
        "@context": "https://schema.org",
        "@graph": [
            {
                "@type": "TechArticle",
                "@id": url + "#article",
                "headline": title,
                "description": desc,
                "url": url,
                "inLanguage": "en",
                "author": {"@id": SITE + "#author"},
                "isPartOf": {"@id": SITE + "#website"},
                "about": {"@id": SITE + "#image"},
                "proficiencyLevel": "Beginner",
                "programmingLanguage": name,
            },
            {
                "@type": "BreadcrumbList",
                "itemListElement": [
                    {"@type": "ListItem", "position": 1, "name": "AWS Emulator Docker Image", "item": SITE},
                    {"@type": "ListItem", "position": 2, "name": "Guides", "item": SITE + "#guides"},
                    {"@type": "ListItem", "position": 3, "name": name, "item": url},
                ],
            },
        ],
    }
    others = " ".join(
        f'<a href="../{s}/"{" aria-current=\"page\"" if s == lang else ""}>{html.escape("Go (Golang)" if s == "go" else n.split(" and ")[0])}</a>'
        for s, n, *_ in LANGUAGES
    )
    e = html.escape
    return f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{e(title)}</title>
<meta name="description" content="{e(desc)}">
<meta name="author" content="Yurii Serhiichuk">
<meta name="robots" content="index, follow, max-image-preview:large">
<meta name="theme-color" content="#ff9f1c" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#0d1117" media="(prefers-color-scheme: dark)">
<meta name="color-scheme" content="light dark">
<meta property="og:site_name" content="AWS Emulator Docker Image">
<meta property="og:locale" content="en_US">
<meta property="og:title" content="{e(title)}">
<meta property="og:description" content="{e(desc)}">
<meta property="og:type" content="article">
<meta property="og:url" content="{url}">
<meta property="og:image" content="{SITE}social-preview.png">
<meta property="og:image:alt" content="The AWS Emulator, ready to run in Docker">
<meta property="og:image:width" content="1280">
<meta property="og:image:height" content="640">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{e(title)}">
<meta name="twitter:description" content="{e(desc)}">
<meta name="twitter:image" content="{SITE}social-preview.png">
<link rel="canonical" href="{url}">
<link rel="icon" href="../../favicon.svg" type="image/svg+xml">
<link rel="icon" href="../../favicon-32.png" sizes="32x32" type="image/png">
<link rel="apple-touch-icon" href="../../apple-touch-icon.png">
<link rel="author" href="https://serhiichuk.dev">
<link rel="alternate" type="text/markdown" title="LLM summary" href="../../llms.txt">
<script type="application/ld+json">
{json.dumps(ld, indent=1)}
</script>
{style}
</head>
<body id="top">
<a class="skip" href="#content">Skip to content</a>
<header class="top"><div class="wrap">
  <div class="bar">
    <a class="brand" href="../../" aria-label="aws-emulator-docker, home page"><img src="../../favicon.svg" width="28" height="28" alt=""><span>aws-emulator-docker</span></a>
    <div class="top-actions">
      <button class="icon-btn" id="theme" type="button" aria-label="Switch to light theme" title="Switch theme">{theme_svg}</button>
      <a class="icon-btn" href="{TREE}examples/{lang}" aria-label="{e(name)} example on GitHub">{github_svg}</a>
    </div>
  </div>
  <nav class="sections" aria-label="Guides">
    {others}
  </nav>
</div></header>

<main class="wrap" id="content" tabindex="-1">
<nav aria-label="Breadcrumb" class="muted"><a href="../../">AWS Emulator Docker Image</a> / <a href="../../#guides">Guides</a> / {e(name)}</nav>
<h1>{e(title)}</h1>
<p>{e(desc)}</p>
<p>Start the emulator with the <a href="../../#quick-start">quick start</a>, or use the
<a href="../../#github-action">GitHub Action</a> in CI.</p>
{body}
</main>

<footer><div class="wrap">
  Created and maintained by <a href="https://serhiichuk.dev">Yurii Serhiichuk</a> (<a href="https://github.com/xSAVIKx">@xSAVIKx</a>).
  Source: <a href="https://github.com/xSAVIKx/aws-emulator-docker">xSAVIKx/aws-emulator-docker</a>.
  Upstream: <a href="https://github.com/aws/aws-emulator">aws/aws-emulator</a>.
  <a href="https://xsavikx.github.io/">More projects</a>.
</div></footer>

<a class="icon-btn to-top" href="#top" aria-label="Back to top"><svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5M5 12l7-7 7 7"/></svg></a>
<div class="sr-only" role="status" aria-live="polite" id="announce"></div>
{script}
</body>
</html>
"""


def main():
    repo, site = sys.argv[1], sys.argv[2]
    root = os.path.dirname(os.path.abspath(site.rstrip("/")))
    index = open(os.path.join(site, "index.html"), encoding="utf-8").read()
    style = re.search(r"<style>.*?</style>", index, re.S).group(0)
    # The last inline script of index.html (the theme and copy buttons).
    start = index.rindex("<script>")
    script = index[start:index.index("</script>", start) + len("</script>")]
    theme_svg = re.search(r'<button class="icon-btn" id="theme"[^>]*>(<svg.*?</svg>)', index, re.S).group(1)
    github_svg = re.search(r'aria-label="Source on GitHub">(<svg.*?</svg>)', index, re.S).group(1)
    urls = []
    for lang, name, framework, sdk, tc_name in LANGUAGES:
        parts = [section(f"examples/{lang}/README.md", repo, root)]
        tc = f"testcontainers/{lang}/README.md"
        has_tc = os.path.exists(os.path.join(root, tc))
        if has_tc:
            parts.append(section(tc, repo, root))
        out_dir = os.path.join(site, "guides", lang)
        os.makedirs(out_dir, exist_ok=True)
        with open(os.path.join(out_dir, "index.html"), "w", encoding="utf-8") as f:
            f.write(page(lang, name, framework, sdk, tc_name, "\n".join(parts), has_tc, style, script, theme_svg, github_svg))
        urls.append(f"{SITE}guides/{lang}/")
        print(f"guides/{lang}/: {len(parts)} section(s)")

    sitemap_path = os.path.join(site, "sitemap.xml")
    sitemap = open(sitemap_path, encoding="utf-8").read()
    lastmod = re.search(r"<lastmod>(.*?)</lastmod>", sitemap).group(1)
    entries = "".join(
        f"  <url>\n    <loc>{u}</loc>\n    <lastmod>{lastmod}</lastmod>\n    <changefreq>weekly</changefreq>\n    <priority>0.8</priority>\n  </url>\n"
        for u in urls if u not in sitemap
    )
    sitemap = sitemap.replace("</urlset>", entries + "</urlset>")
    with open(sitemap_path, "w", encoding="utf-8") as f:
        f.write(sitemap)


if __name__ == "__main__":
    main()
