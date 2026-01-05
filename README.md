# go do (dev ops)

Go tool for dev ops: init, build, lint, test, deploy apps.

```bash
# install direnv to automatically manage PATH for your app
brew install direnv

# get the tool and init in the repo
go get -tool github.com/housecat-inc/do
go tool do init --allow

# use the `go do` shorthand for common operations
go do
 → go generate ./...
 → go build ./...
 → go vet ./...
 → go do lint ./...
 → go test ./...

go do deploy
```

## Adding lint rules

To enforce coding standards, we prefer software tools that tell you exactly what standards are not met and where. The `golangci-lint` tool provide this and can be extended.

Contrast this approach to documenting standards in README.md / AGENTS.md / CLAUDE.md, which leaves both developers and LLMs room to interpret and forget.

A better agentic approach is to tell Claude to write code:

> Write a lint plugin that enforces "use github.com/pkg/errors everywhere"
>
> ⏺ I'll explore to understand lint plugins, then create one that enforces the use of the errors packages.
> ⏺ Now I'll create the lint plugin. The analyzer will:
>
> 1. In any file: flag direct use of err. (should use pkg/errors)


## Configuring deploy
