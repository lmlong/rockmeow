---
name: code-review
description: Review code for bugs, security issues, and best practices
metadata: {"nanobot":{"emoji":"🔍"}}
---
# Code Review

Review code changes for quality and issues.

## How to Read Code

Use the `file` tool to read source files:

```json
{
  "operation": "read",
  "path": "/path/to/file.go"
}
```

To list files in a directory:

```json
{
  "operation": "list",
  "path": "/path/to/directory"
}
```

## Review Checklist

### Security
- SQL injection vulnerabilities
- XSS (Cross-Site Scripting)
- Command injection
- Sensitive data exposure
- Authentication/authorization issues
- Hardcoded credentials or secrets

### Code Quality
- Code duplication
- Complex conditional logic
- Missing error handling
- Resource leaks (file handles, connections)
- Race conditions in concurrent code
- Nil/null pointer dereferences

### Best Practices
- Proper naming conventions
- Documentation/comments where needed
- Function/method length
- Test coverage
- SOLID principles adherence

## Review Output Format

Provide a structured review with:

1. **Summary**: Brief overview of the code and its purpose
2. **Critical Issues**: Security vulnerabilities and bugs (must fix)
3. **Warnings**: Potential problems and code smells
4. **Suggestions**: Improvements for readability and maintainability
5. **Positive Aspects**: Good practices observed in the code
