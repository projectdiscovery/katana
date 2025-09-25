# Issue Template Management

## New Issue Workflow (Discussion-First)

To improve issue triage and reduce noise from questions being filed as bugs, all issue creation now goes through GitHub Discussions first.

### For Users:

**❌ Direct issue creation is disabled**
**✅ All reports must start as discussions**

Users will be redirected to:
- 🐛 **Bug Reports** → [Q&A Discussions](https://github.com/projectdiscovery/katana/discussions/new?category=q-a)
- 💡 **Feature Requests** → [Ideas Discussions](https://github.com/projectdiscovery/katana/discussions/new?category=ideas)  
- ❓ **Questions** → [Q&A Discussions](https://github.com/projectdiscovery/katana/discussions/new?category=q-a)

### For Maintainers:

#### Converting Discussions to Issues:

1. **Review the discussion** thoroughly
2. **Determine if it's a valid bug/feature** (not just a question)
3. **Convert to issue** using GitHub's "Convert to Issue" feature:
   - Go to the discussion
   - Click "⋯" menu → "Convert to issue"
   - Add appropriate labels and assignees
   
#### Triage Guidelines:

**Convert to Issue:**
- ✅ Confirmed bugs with reproduction steps
- ✅ Well-defined feature requests with clear use cases
- ✅ Security vulnerabilities (after initial assessment)

**Keep as Discussion:**
- ❌ Usage questions ("How do I...?")
- ❌ Configuration help
- ❌ Unclear or incomplete bug reports
- ❌ Feature ideas that need more discussion/refinement

### Benefits:

- 📊 **Better issue quality** - Only confirmed bugs/features become issues
- 🎯 **Easier triage** - Questions don't clutter the issue tracker
- 💬 **Community involvement** - Discussion encourages collaboration before formal issues
- 🧹 **Cleaner issue tracker** - Focus on actionable items only

## Re-enabling Templates (If Needed)

If you need to temporarily re-enable direct issue creation:

```bash
# Re-enable templates
mv issue-report.md.disabled issue-report.md
mv feature_request.md.disabled feature_request.md

# Update config.yml to add them back
```

## Template Files:

- `config.yml` - Main configuration (redirects all to discussions)
- `issue-report.md.disabled` - Bug report template (disabled)
- `feature_request.md.disabled` - Feature request template (disabled)
