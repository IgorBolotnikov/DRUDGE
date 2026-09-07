# Terminology

The words below mean exactly one thing each. Use them in code, docs, commit messages and output.

## Drudger

An agent running in a reusable sandbox that does the work. One Drudger is one agent in one sandbox.

A Drudger is either occupied with a Task or idle. It goes idle when a Session ends, whether that Session got shit done or fucked up, and it stays around afterwards ready for the next Task. Drudgers are created as they are needed and live until the pool is deliberately shrunk.

The agent and the sandbox break independently, so a Drudger has one state for each of them. A perfectly good sandbox can hold an agent the vendor turned away.

- **Sandbox health** is usable, gone, or mounted on the wrong workspace. It is learned from the sandbox listing every time a Drudger is put to work.
- **Agent health** is ready or refused. Refused means the vendor turned the agent away, so it can do nothing until whatever the vendor named is fixed. It is learned from the Session a Drudger finished.

A Drudger answers whether it is able to work. How the work itself is going is a Session question.

## Session

A single run of work on a Task by one Drudger.

A Session is what the agent harness resumes from, and its id comes from the init event the agent writes. A Task can be worked on many times, so a Task can have many Sessions over its life. Only the last one is tracked.

A Drudger hosts at most one Session at a time.

A Session that the vendor turned away never got going. The agent was refused before it could work, so the Task is not at fault and goes back where it came from.

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

- **Is the Drudger able to work?** Answered by the Drudger record, in two parts: its sandbox and its agent.
- **How is the work going?** Answered by the Session's run directory.
