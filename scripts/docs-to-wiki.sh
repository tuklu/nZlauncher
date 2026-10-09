#!/usr/bin/env bash
# Builds GitHub wiki pages from README.md + docs/. docs/ stays the source of
# truth; the wiki is regenerated from it by .github/workflows/wiki.yml.
#
#   scripts/docs-to-wiki.sh <repo-root> <wiki-dir>
set -euo pipefail

repo=${1:?repo root}
wiki=${2:?wiki dir}
blob="https://github.com/tuklu/nZlauncher/blob/main"

# docs file -> wiki page name, in sidebar order
pages=(
    "usage.md:Usage"
    "configuration.md:Configuration"
    "hyprland.md:Hyprland"
    "plugins.md:Plugins"
    "how-it-works.md:How-it-works"
    "development.md:Development"
)

page_map=""
for entry in "${pages[@]}"; do
    page_map+="${entry%%:*}=${entry#*:};"
done

# Wiki pages are flat: docs/x.md and x.md become the page name, anchors kept.
# Any other relative link points back into the repo on GitHub.
# $2 is where the source file lives relative to the repo root ("" or "docs/").
rewrite_links() {
    PAGE_MAP="$page_map" BLOB="$blob" SRC_DIR="$2" perl -pe '
        BEGIN {
            %page = map { split /=/ } split /;/, $ENV{PAGE_MAP};
        }
        s{\]\(([^)\s]+)\)}{
            my $target = $1;
            my ($path, $anchor) = split /#/, $target, 2;
            $anchor = defined $anchor ? "#$anchor" : "";
            if ($target =~ m{^(https?:|mailto:|#)}) {
                "]($target)";
            } else {
                (my $name = $path) =~ s{^docs/}{};
                exists $page{$name}
                    ? "]($page{$name}$anchor)"
                    : "]($ENV{BLOB}/$ENV{SRC_DIR}$path$anchor)";
            }
        }ge;
    ' "$1"
}

mkdir -p "$wiki"

rewrite_links "$repo/README.md" "" > "$wiki/Home.md"

for entry in "${pages[@]}"; do
    rewrite_links "$repo/docs/${entry%%:*}" "docs/" > "$wiki/${entry#*:}.md"
done

{
    echo "**[Home](Home)**"
    echo
    for entry in "${pages[@]}"; do
        name=${entry#*:}
        echo "- [${name//-/ }]($name)"
    done
} > "$wiki/_Sidebar.md"

cat > "$wiki/_Footer.md" <<EOF
Generated from [\`docs/\`](https://github.com/tuklu/nZlauncher/tree/main/docs). Edit there, changes here get overwritten.
EOF
