## Running task implementation agent

There are a bunch of thinks which the agent does not need to do itself. They should be handled by simple code. We need to narrow down the role of the agent in the workflow. Let the agent be involved only in the parts where LLM inference is the only way we can achieve the goal.

- Pick the task for it. It's deterministic so there is no point in making the agent try to choose it itself
- Run `git stash` and `git checkout dev` and `git pull` because the agent tries running 9000 bash commands just to understand what the hell is going on with the branches. Make sure it's done for all repos involved in the task
- We know which task to pick, so we know how to name the branch: `feat/<ticket number>/<task name from the md file>`. Spare the clanker from doing it time and again
- Make it clear in the prompt that the target branch is always `dev`
- Take a timestamp when the task is moved to "in-progress". And move it in progress automatically — don't make the clanker do it, it's a waste of tokens
- Make sure it knows how to write comments, how to cleanup the slop, how to write proper PRs
