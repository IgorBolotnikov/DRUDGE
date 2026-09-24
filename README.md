<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="DRUDGE" />
  </picture>
</p>

<!--toc:start-->

- [DRUDGE](#drudge)
  - [Install](#install)
  - [Main features](#main-features)
  - [Other improvements - idea dump](#other-improvements-idea-dump)
  <!--toc:end-->

DRUDGE is a self-hosted control plane for coding agents.

Give it a backlog. It gives the work to isolated agents, watches them work, runs the checks, and keeps going.

They're not coworkers. They're not digital employees. They're tools.

## Install

DRUDGE runs on Linux and macOS. On Windows, use WSL.

```sh
curl -fsSL https://raw.githubusercontent.com/IgorBolotnikov/DRUDGE/main/install.sh | sh
```

The script puts `drg` in `~/.local/bin`, or in `/usr/local/bin` when run as root. Set `DRG_VERSION=v0.1.0` to install a specific release, or `DRG_INSTALL_DIR` to pick another directory.

With Go installed, this works too:

```sh
go install github.com/IgorBolotnikov/DRUDGE/cmd/drg@latest
```

Then run `drg setup`.

## Main features

- Create a directory with a global config, projects, PRDs, tasks
- Add CLI for managing everything
- Provide config from the start
- Provide themes from the start
- Customize the clankers (or drudgers?)
- Animations, because why not make it actually fun to work with?

## Other improvements - idea dump

- Extensibility with plugins
