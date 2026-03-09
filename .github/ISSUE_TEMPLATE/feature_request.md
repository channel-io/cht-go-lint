---
name: Feature Request / New Rule
about: Suggest a new rule or feature
title: "[Feature] "
labels: enhancement
assignees: ''
---

## Summary

<!-- Brief description of the rule or feature -->

## Proposed rule (if applicable)

- **Name**: `category/rule-name`
- **Category**: <!-- dependency, naming, interface, structure, ddd -->
- **Tier**: <!-- universal, layer-aware, component-aware, domain-specific -->

## Example

### Code that should be flagged

```go
// example of code that violates the rule
```

### Code that should pass

```go
// example of code that satisfies the rule
```

## Motivation

<!-- Why is this rule or feature useful? -->

## Configuration

```yaml
# How users would configure this rule
rules:
  category/rule-name:
    severity: error
    options:
      key: value
```
