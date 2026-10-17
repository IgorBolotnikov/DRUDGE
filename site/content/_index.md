---
title: DRUDGE
layout: hextra-home
---

<div class="hx:mt-6 hx:mb-8">
<img src="/images/logo.svg" alt="DRUDGE" width="400" height="96" />
</div>

<div class="hx:mb-6">
{{< hextra/hero-headline >}}
  DRUDGE runs your backlog through coding agents, each in its own sandbox, on your own machine.
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-8">
{{< hextra/hero-subtitle >}}
  You make the decisions and review what comes back. The agents do the grunt work in between.<br /><br />
  They're not coworkers. They're not digital employees. They're tools.
{{< /hextra/hero-subtitle >}}
</div>

```sh
curl -fsSL https://drgtools.dev/install.sh | sh
```

<div class="hx:mt-6 hx:mb-12">
{{< hextra/hero-button text="Get started" link="/docs/getting-started/" >}}
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card title="Projects" subtitle="Projects that link a local directory and its git repositories" >}}
  {{< hextra/feature-card title="Tasks" subtitle="Tasks stored as plain markdown files" >}}
  {{< hextra/feature-card title="Dependencies" subtitle="Task dependencies, ensuring work starts only once its blockers are done" >}}
  {{< hextra/feature-card title="Tickets and parents" subtitle="Tasks grouped by ticket (external ID) and by parent task" >}}
  {{< hextra/feature-card title="Drudgers" subtitle="A pool of reusable drudgers, each with its own git worktree per repository" >}}
  {{< hextra/feature-card title="Parallel" subtitle="Several drudgers working in parallel, you configure how many" >}}
  {{< hextra/feature-card title="Claude Code" subtitle="Claude Code as the agent vendor (others TBD)" >}}
  {{< hextra/feature-card title="Run records" subtitle="A full record of every run: the prompt, the event stream, the logs, the cost and the time it took to work on a task" >}}
  {{< hextra/feature-card title="Refusals" subtitle="Tasks go back to the queue when the vendor turns an agent away, whether by rate limit, outage or expired login" >}}
  {{< hextra/feature-card title="Stuck sandboxes" subtitle="Commands to reclaim a stuck sandbox or nuke a broken one" >}}
  {{< hextra/feature-card title="Planning skill" subtitle="A bundled skill that lets a planning agent write DRUDGE tasks for you" >}}
  {{< hextra/feature-card title="Themes and config" subtitle="Color themes and a config file with a JSON schema" >}}
{{< /hextra/feature-grid >}}
