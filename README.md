###### _<div align="right"><sub>// made with <3</sub></div>_

<div align="center">

<!-- Project Banner -->

<a href="https://github.com/LatteSec/ctfjx">
  <img src="https://github.com/LatteSec/ctfjx/blob/main/www/docs/static/logo.png" width="300" height="300" alt="">
</a>

<br>

<!-- Badges -->

![badge-workflow]
[![badge-license]][license]
![badge-language]
[![badge-pr]][prs]
[![badge-issues]][issues]

<br><br>

<!-- Description -->

Your preferred CTF platform's logger.

<br><br>

---

<!-- TOC -->

**[<kbd> <br> Quick Start <br> </kbd>](#quick-start)**
**[<kbd> <br> Thanks <br> </kbd>](#special-thanks)**
**[<kbd> <br> Contribute <br> </kbd>][contribute]**

---

<br>

</div>

# About

This logger aims to be performant and async out of the gate.

## Features

- Log rotations
- Async logging
- File logging

## Usage

### Install into your project

```sh
go get -u github.com/LatteSec/log@latest
```

### Use it

```go
import "github.com/LatteSec/log"

func main() {
  defer log.Close()

  // Use it straight away with a default logger
  log.Info().Msg("Hello, World!")
  log.Log(log.INFO).Msg("Hello, World!")

  // or create a logger
  logger := log.NewLogger().
              Name("my-logger").
              Level(log.INFO)

  logger.Start()
  logger.Info().Msg("Hello from custom logger!")
  logger.Log(log.INFO).Msg("Hello from custom logger!")

  // and you can register it to the global logger too!
  log.Register(logger)
}
```

## Development

```sh
# For all the commands you will need
make help
```

## Special Thanks

- **[Waku][waku]** - _For the project templating_
- **[Img Shields][img-shields]** - _For the awesome README badges_
- **[Hyprland][hyprland]** - _For showing how to make beautiful READMEs_
- **[Hyprdots][hyprdots]** - _For showing how to make beautiful READMEs_

---

![stars-graph]

<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->

[stars-graph]: https://starchart.cc/LatteSec/log.svg?variant=adaptive
[prs]: https://github.com/LatteSec/log/pulls
[issues]: https://github.com/LatteSec/log/issues
[license]: https://github.com/LatteSec/log/blob/main/LICENSE

<!---------------- {Links} ---------------->

[contribute]: https://github.com/LatteSec/log/blob/main/CONTRIBUTING.md

<!---------------- {Thanks} ---------------->

[waku]: https://github.com/caffeine-addictt/waku
[hyprland]: https://github.com/hyprwm/Hyprland
[hyprdots]: https://github.com/prasanthrangan/hyprdots
[img-shields]: https://shields.io

<!---------------- {Badges} ---------------->

[badge-workflow]: https://github.com/LatteSec/log/actions/workflows/test-worker.yml/badge.svg
[badge-issues]: https://img.shields.io/github/issues/LatteSec/log
[badge-pr]: https://img.shields.io/github/issues-pr/LatteSec/log
[badge-language]: https://img.shields.io/github/languages/top/LatteSec/log
[badge-license]: https://img.shields.io/github/license/LatteSec/log
