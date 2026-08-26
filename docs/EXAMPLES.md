# Examples

This is just a collection of example of how the vibe of this project should look like.

## General terminology

How to approach the wording
"Call things what they physically are. The machine did something, so tell me what it did."
worker → a running agent process
drudge → an agent instance
sandbox → isolated execution environment
job → a unit of work
run → an execution
log → logs
output → output
failure → failure
retry → retry
kill → terminate a worker
queue → queue
horde → collection of workers

## Vague ideas about the dashboard

```
DRUDGE
────────────────────────────────────────────

HORDE                         BACKLOG
12 working                    47 queued
 3 reviewing                   8 blocked
 1 fucked up                   6 ready

────────────────────────────────────────────

WORKING

#1842  Add OAuth callback          drudge-7   14m
#1847  Fix flaky integration test  drudge-3    8m
#1851  Refactor config loader      drudge-9   31m
#1854  Add pagination              drudge-12   4m

────────────────────────────────────────────

RECENTLY

✓ #1839  got shit done
✓ #1840  got shit done
✗ #1841  fucked up
⚠ #1843  needs babysitting
☠ #1844  was terminated
✓ #1845  is back from the dead
```

```
#1841  Fix authentication race

DRUDGE-4
sandbox: /drudge/sandboxes/1841
branch: drudge/1841-auth-race

14:02  cloned repository
14:03  read ticket
14:04  inspected auth middleware
14:07  changed middleware.go
14:08  running tests
14:09  tests failed
14:10  attempted fix
14:11  tests failed
14:12  gave up

RESULT: fucked up

reason:
  TestAuthRefreshRace failed 3 times.

[ RESTART ] [ KILL ] [ TAKE OVER ]
```

Dos and dont's in agent statuses:

```
Don't show:

> Agent encountered an unexpected execution failure.

Show:

> 1 has fucked up

Don't show:

> Agent requires human intervention.

Show:

> 3 need babysitting

Don't show:

> Agent successfully completed task.

Show:

> 7 got shit done
```
