# Maintainer Guide: Discussion-First Issue Management

## Overview

Katana now uses a **discussion-first approach** for issue management to improve triage quality and reduce noise from questions being filed as bugs.

## How It Works

### 1. **All Reports Start as Discussions**
- Users cannot create issues directly
- All bug reports → **Q&A Discussions**  
- All feature requests → **Ideas Discussions**
- All questions → **Q&A Discussions**

### 2. **Automated Triage Helper**
- Auto-responds to discussions with helpful guidance
- Auto-flags potential bugs with keywords detection
- Provides checklists for proper bug reporting

### 3. **Maintainer Conversion Process**
- Review discussions for completeness
- Convert valid issues using GitHub's built-in feature
- Apply appropriate labels during conversion

## Conversion Guidelines

### 🐛 **Bug Reports** → Convert to Issue When:

**Well-Defined Problems:**
- Clear reproduction steps provided
- Katana version specified
- Expected vs actual behavior described  
- Environment details included
- Error messages/logs included

**Confirmed Bugs:**
- Issue reproduced by maintainer or community
- Not a configuration/usage question
- Not working as designed

**Keep as Discussion:**
- Incomplete information
- Usage questions ("How do I...?")
- Configuration problems
- Working as intended

### 💡 **Feature Requests** → Convert to Issue When:

**Solid Proposals:**
- Clear use case defined
- Benefits to community explained
- Implementation approach considered
- Not easily achievable with existing features

**Community Support:**
- Multiple users expressing interest
- Maintainer approval for implementation
- Fits project roadmap

**Keep as Discussion:**
- Vague ideas needing refinement
- Better suited as external tools/plugins
- Conflicts with project goals
- Needs more community input

## Conversion Process

### Using GitHub's Convert Feature:

1. **Open the discussion**
2. **Click the "⋯" menu** (top right)
3. **Select "Convert to issue"**
4. **Choose repository** (same repo)
5. **Review title/body** - edit if needed
6. **Add labels:**
   - `Type: Bug` for confirmed bugs
   - `Type: Enhancement` for approved features  
   - `Priority: High/Medium/Low` as appropriate
   - `Component: Engine/Parser/Output` etc.

### Template for Converted Issues:

When converting, consider adding this note:

```markdown
**Converted from Discussion:** #[discussion_number]

<!-- Original discussion provided community input and initial triage -->

[Original discussion content here]

---

**Maintainer Notes:**
- [ ] Issue confirmed through discussion
- [ ] Reproduction steps verified  
- [ ] Ready for implementation/investigation
```

## Workflow Benefits

### **For Project Health:**
- **Cleaner issue tracker** - Only actionable items
- **Better metrics** - Issues vs discussions clearly separated
- **Faster resolution** - Less time sorting questions from bugs

### **For Community:**
- **Inclusive discussions** - Everyone can participate in triage
- **Better help** - Community can answer questions quickly  
- **Learning opportunity** - Users see resolution process

### **For Maintainers:**
- **Pre-filtered issues** - Only valid bugs/features reach issue tracker
- **Rich context** - Discussion history provides background
- **Community input** - Others help validate before conversion

## Examples

### **Good Bug Discussion → Issue Conversion**

**Discussion Title:** "Katana crashes when using -hl with custom headers"

**Discussion Body:** 
- Katana version: v1.2.1
- Command: `katana -u example.com -hl -H "Custom: value"`
- Error: panic in hybrid engine
- Platform: macOS 14.1
- Reproduction: consistent crash

**→ Convert to Issue:** Clear bug with reproduction steps

### **Question → Keep as Discussion**

**Discussion Title:** "How to crawl only PDF files?"

**Discussion Body:**
- New user question
- Asking for usage help
- Not a bug or feature request

**→ Keep as Discussion:** Usage question, answer in discussion

### **Needs More Info → Keep as Discussion**

**Discussion Title:** "Katana doesn't work"

**Discussion Body:**
- Vague description
- No version, command, or error details
- No reproduction steps

**→ Keep as Discussion:** Request more information first

## Quick Reference

| Type | Action | Labels for Conversion |
|------|---------|-------------------|
| **Confirmed Bug** | Convert → Issue | `Type: Bug`, `Priority: [level]` |
| **Approved Feature** | Convert → Issue | `Type: Enhancement`, `Priority: [level]` |  
| **Usage Question** | Keep → Discussion | N/A |
| **Needs Info** | Keep → Discussion | N/A |
| **Security Issue** | Email → security@projectdiscovery.io | N/A |

This workflow ensures high-quality issues while maintaining an inclusive, helpful community environment!
