# Terminology

The words below mean exactly one thing each. Use them in code, docs, commit messages and output.

## Drudger

A reusable sandbox that does the work. One Drudger is one sandbox.

A Drudger is either occupied with a Task or idle. It goes idle when a Session ends, whether that Session got shit done or fucked up, and it stays around afterwards ready for the next Task. Drudgers are created as they are needed and live until the pool is deliberately shrunk.

A Drudger's own state is about the sandbox: usable, gone, or mounted on the wrong workspace. How the work inside it is going is not a Drudger question.

## Session

A single run of work on a Task by one Drudger.

A Session is what the agent harness resumes from, and its id comes from the init event the agent writes. A Task can be worked on many times, so a Task can have many Sessions over its life. Only the last one is tracked.

A Drudger hosts at most one Session at a time.

## Task

A piece of work to be done.

A Task carries its own status and the record of its last Session. It does not carry which Drudger is working on it. That relation is the Drudger's to know.

## How they relate

```
Drudger  --- hosts at most one --->  Session  --- is one run of --->  Task
   |                                                                    |
   +----- occupied by, or idle -----------------------------------------+

Task --- has had many Sessions, only the last is tracked ---> Session
```

Two questions with two homes, and they never overlap:

- **Is the Drudger usable?** Answered by the Drudger record.
- **How is the work going?** Answered by the Session's run directory.

## A note on the code

The code still says `Runner` where this document says `Drudger` — `RunnerService`, `RunnerID`, `formatRunnerName`, `internal/runner`, and the `maxConcurrentRunners` config key. The rename is pending. New code and new docs use the words above.
