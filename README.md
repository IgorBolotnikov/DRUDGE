<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="DRUDGE" />
  </picture>
</p>

<!--toc:start-->

- [Install](#install)
  - [Single command install](#single-command-install)
  - [Go install](#go-install)
- [Future improvements](#future-improvements)

<!--toc:end-->

DRUDGE runs your backlog through coding agents, each in its own sandbox, on your own machine.

You make the decisions and review what comes back. The agents do the grunt work in between.

They're not coworkers. They're not digital employees. They're tools.

## Install

DRUDGE runs on Linux and macOS. On Windows, use WSL.

### Single command install

```sh
curl -fsSL https://raw.githubusercontent.com/IgorBolotnikov/DRUDGE/main/install.sh | sh
```

The script puts `drg` in `~/.local/bin`, or in `/usr/local/bin` when run as root. Set `DRG_VERSION=v0.1.0` to install a specific release, or `DRG_INSTALL_DIR` to pick another directory.

### Go install

```sh
go install github.com/IgorBolotnikov/DRUDGE/cmd/drg@latest
```

Then run `drg setup`.

## Future improvements

- Extensibility with plugins
