---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg drudger reclaim
linkTitle: reclaim
weight: 3
generated: true
---

```text
usage: drg drudger reclaim
```

Free the Drudger slots that are still claimed by an agent that is gone.

A Drudger frees its slot when its agent writes an exit file. An agent killed before that leaves the slot claimed, and a claimed slot counts against the concurrency limit.

This only rewrites the Drudgers file. No process is killed, no sandbox is removed and the tasks those slots held keep their status, so start one over with drg task rerun.
