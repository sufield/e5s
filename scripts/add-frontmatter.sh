#!/bin/bash
set -euo pipefail

# Add Diátaxis frontmatter to documentation files

add_frontmatter() {
    local file="$1"
    local type="$2"
    local audience="$3"

    # Check if file already has frontmatter
    if head -1 "$file" | grep -q "^---$"; then
        echo "  ⏭️  $file (already has frontmatter)"
        return
    fi

    # Create temp file with frontmatter + original content
    {
        echo "---"
        echo "type: $type"
        echo "audience: $audience"
        echo "---"
        echo ""
        cat "$file"
    } > "$file.tmp"

    mv "$file.tmp" "$file"
    echo "  ✅ $file"
}

echo "📝 Adding Diátaxis frontmatter to documentation files..."
echo ""

# Tutorials (learning-oriented, beginner)
echo "🎓 Tutorials:"
for file in docs/tutorials/*.md; do
    [ -f "$file" ] && add_frontmatter "$file" "tutorial" "beginner"
done

# How-to guides (task-oriented, intermediate)
echo ""
echo "🔧 How-To Guides:"
for file in docs/how-to-guides/*.md; do
    [ -f "$file" ] && add_frontmatter "$file" "how-to" "intermediate"
done

# Reference (information-oriented, intermediate)
echo ""
echo "📖 Reference:"
for file in docs/reference/*.md; do
    [ -f "$file" ] && add_frontmatter "$file" "reference" "intermediate"
done

# Explanation (understanding-oriented, advanced for architecture, intermediate for others)
echo ""
echo "💡 Explanation:"
for file in docs/explanation/*.md; do
    if [[ "$file" == *"ARCHITECTURE"* ]] || [[ "$file" == *"DESIGN_BY_CONTRACT"* ]]; then
        add_frontmatter "$file" "explanation" "advanced"
    else
        add_frontmatter "$file" "explanation" "intermediate"
    fi
done

echo ""
echo "✅ Frontmatter added successfully!"
echo ""
echo "Next steps:"
echo "  1. Review the changes: git diff docs/"
echo "  2. Adjust audience levels if needed"
echo "  3. Commit: git add docs/ && git commit -m 'docs: Add Diátaxis metadata headers'"
