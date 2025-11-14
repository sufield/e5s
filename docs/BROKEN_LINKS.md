# Broken Links Report

## Testing

Run the link checker to verify current state:

```bash
make check-links
```

Or run lychee directly:

```bash
lychee '**/*.md' --exclude-loopback --max-concurrency 128 --timeout 20 --format detailed
```

## CI/CD Status

✅ GitHub Actions workflow configured at `.github/workflows/links.yml`
✅ Smart failure logic: Only fails on internal link errors
✅ Weekly automated checks enabled
✅ Auto-creates issues for broken links on main branch
