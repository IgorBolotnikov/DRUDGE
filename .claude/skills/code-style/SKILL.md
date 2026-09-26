---
name: code-style
description: Code, comment, prose and commit style for the DRUDGE repo. Load before writing or editing any Go file, including tests, any markdown file, and before writing a commit message or a branch name.
---

These rules apply to every Go file in this repo, production code and tests alike. The prose rules also apply to markdown files. Read them before you write, and check your diff against them before you finish.

## Code style

- Write table-driven tests, structured so new cases are very easy to add.
- Write tests for public API (exported methods, functions, etc). Do not test every single private function because that pins the implementation details and kills the refactoring flexibility. Thus public API tests should cover all the cases.
- Use full, readable names for variables — `service`, not `s`. Avoid single-letter names outside of trivial loop indices. This contradicts the Go style guide, but we want to make the code actually readable
- When a string carries meaning (a status value, a key, a flag name, a file name pattern), pull it into a named const. Reserve inline string literals for plain human-readable messages (log lines, error text).
- We value good developer experience. Explicit hard errors with good error messages are always preferred over trying to provide some default behavior and hide the incorrect behavior. We want to always provide proper feedback to the user so that they have as little WTF moments as possible. This also solves many support tickets which we want to avoid as much as possible.
- Commit rules (non-negotiable): use conventional commits, commit message is single line, clear and short. Adding walls of text in commit message is a huge red flag.
- Branch rules (non-negotiable): use conventional commit branch prefixes: `<prefix>/<branch-name>`.
- Attribution rules (non-negotiable): never credit Claude, Anthropic or any other AI tool anywhere. That covers `Co-Authored-By` trailers, "Generated with" footers, code comments, docs and changelogs. The work belongs to the person who ran the tool.
- boolean variables should read like questions to be answered with "yes" or "no". Use `is/has/does/should` and other name prefixes to formulate the question. Idiomatic go names are allowed: `ok`, and the `want*`/`got` fields of table-driven tests. The rule covers test files too, including struct fields, function parameters and named return values. Functions that return a bool are not variables and keep their names.

Examples of boolean variables:

```go
// BAD -> GOOD
ready -> isReady
landed -> hasLanded
compiles -> doesCompile
delete -> shouldDelete
wrote -> didWrite
called -> wasCalled
```

## Prose in markdown

This covers `docs/*.md`, `README.md`, `CLAUDE.md`, skills and any other markdown in the repo.

- Follow the "How to write one" rules for comments below. Plain, short sentences. No semicolons. No "X, not Y", "rather than Z" or "instead of Z".
- Do not hard-wrap paragraphs. Write one line per paragraph and one line per bullet, however long. Renderers wrap the text themselves. Put line breaks only where they carry meaning: between paragraphs, between list items and inside code blocks.

## Comments and documentation comments style guide

A comment earns its place when it saves the reader work. Test every sentence you write: can a reader point at the code or the behaviour that makes it true? If the answer is no, delete the sentence.

**How to write one**

- Use simple, direct, technical English. The readers are busy engineers and many of them are non-native English speakers. Anything that makes them read a sentence twice is a defect.
- Write for the engineer reading the file. An AI agent is not the audience.
- Make a thing from the code the subject of the sentence: a function, a file, a field, a status, a process. Say what it does in the present tense.
- State conditions and mechanisms. Rewrite any comment that reads as a story about what happens to whom.
- Name the number or the mechanism the code actually uses. Words like "moments", "shortly", "a while" and "quickly" carry no information.
- Keep sentences short. Avoid semicolons and long chains of clauses.
- Give the reason directly and positively. Avoid the "X, not Y" construction and its trailing forms, "rather than Z" and "instead of Z".
- Name a failure state flatly. Drop the words that make it sound worse than it is: "forever", "no way out", "with nothing left to free it".
- Avoid rhetorical shapes. Emphasis by negation ("proves nothing"), emphasis by repetition ("bookkeeping and nothing else") and rhetorical questions all cost the reader a second pass.

**What to leave out**

- Do not reference files or directories by path, because that is as bad as hardcoding them. Use the plain name of the thing. Example: "~/.drudge/config.json" -> "global config file". Go identifiers are welcome, because a reader can grep for them.
- Do not restate the signature in English. Write what the reader cannot see: the constraint, the failure the code guards against, the thing it deliberately leaves alone.
- Do not repeat the same comment in more than one place, because that is as bad as copy-pasting code. Put the fact where it lives.
- Delete the comment when the name already says it. This bites hardest on test helpers and named consts.

**Documentation comments**

- Start with the name of the thing and say what it does for the caller. Cover what it returns, what it writes and what it refuses.
- Say what the code does and stop. One or two sentences is the normal length.
- Cut a trailing "so ..." or "because ..." clause that names the benefit. A reader gets the benefit from the code.
- Keep a trailing clause when it protects the code from a change that looks like a simplification. Test it: could someone delete the clause, write the code the obvious way, and break something?
- Logic that belongs to another function belongs in that function's doc, unless the test above keeps it here.
- A doc comment longer than the code it documents means the code should be split.

**Examples**

Narrative register:

```go
// A slot is claimed for as long as its agent works, and the agent frees it by
// writing an exit file on its way out. An agent that dies without getting
// there leaves the slot claimed with nothing that will ever free it.
```

```go
// A slot is claimed by a launch and freed once its run directory holds an exit
// file. The last line of the launcher script writes that file, so an agent
// killed before that line leaves the slot claimed.
```

Vague quantity:

```go
// A launch makes the run directory moments after it claims a slot.
```

```go
// A launch creates the run directory right after it claims the slot, so a
// claim older than the grace period has no launch behind it.
```

Trailing clause:

```go
// formatAgo renders roughly how long ago a moment was, at the precision a
// reader skimming a listing needs.
```

```go
// formatAgo renders roughly how long ago a moment was.
```

Drama in a failure state:

```go
// an agent killed before that line leaves the slot claimed with nothing left
// to free it. Every stuck slot lowers how many Drudgers the project can run,
// until each launch fails on the concurrency limit.
```

```go
// an agent killed before that line leaves the slot claimed. A claimed slot
// counts against the concurrency limit.
```

A comment the name already covers:

```go
// idleDrudger is a Drudger that exists and holds no task.
func idleDrudger(slot int) *Drudger {
```

```go
func idleDrudger(slot int) *Drudger {
```

A trailing clause that earns its place, because reordering the replacements breaks escaping:

```go
// frontMatterEscaper folds a value onto a single line. The backslash is
// replaced first, so an escape it introduces is not escaped again.
```
