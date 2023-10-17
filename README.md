# Opinionated zerolog configuration

[![pkg.go.dev](https://pkg.go.dev/badge/gitlab.com/tozd/go/zerolog)](https://pkg.go.dev/gitlab.com/tozd/go/zerolog)
[![Go Report Card](https://goreportcard.com/badge/gitlab.com/tozd/go/zerolog)](https://goreportcard.com/report/gitlab.com/tozd/go/zerolog)
[![pipeline status](https://gitlab.com/tozd/go/zerolog/badges/main/pipeline.svg?ignore_skipped=true)](https://gitlab.com/tozd/go/zerolog/-/pipelines)
[![coverage report](https://gitlab.com/tozd/go/zerolog/badges/main/coverage.svg)](https://gitlab.com/tozd/go/zerolog/-/graphs/main/charts)

A Go package providing opinionated [zerolog](https://github.com/rs/zerolog) configuration
and a pretty-printer tool for its logs, `prettylog`.

![Pretty Logging Image](pretty.png)

## Installation

This is a Go package. You can add it to your project using `go get`:

```sh
go get gitlab.com/tozd/go/zerolog
```

[Releases page](https://gitlab.com/tozd/go/zerolog/-/releases)
contains a list of stable versions of the `prettylog` tool.
Each includes:

- Statically compiled binaries.
- Docker images.

You should just download/use the latest one.

The tool is implemented in Go. You can also use `go install` to install the latest stable (released) version:

```sh
go install gitlab.com/tozd/go/zerolog/cmd/go/prettylog@latest
```

To install the latest development version (`main` branch):

```sh
go install gitlab.com/tozd/go/zerolog/cmd/go/prettylog@main
```

## GitHub mirror

There is also a [read-only GitHub mirror available](https://github.com/tozd/go-zerolog),
if you need to fork the project there.
