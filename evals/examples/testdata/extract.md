---
title: "Frontmatter is not a fence"
tags:
  - extraction
---

# Extraction fixture

<!-- eval:skip -->
```
ascii diagram
```

```sh
echo hello
```

```js
console.log(1);
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: v1
kind: Secret
metadata:
  name: second
```

```go
// BAD: this example is deliberately wrong
func main() {
```

```
plain prose that no heuristic matches
```
