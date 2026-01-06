# go do (dev ops)

Go tool for dev ops: init, build, lint, test, deploy apps.

```bash
# install direnv to automatically manage PATH for your app
brew install direnv

# get the tool and init in your go project
go get -tool github.com/housecat-inc/do
go tool do init --allow

# use the `go do` shorthand for common operations
go do
 → go mod tidy
 → go generate ./...
 → go build ./...
 → go vet ./...
 → go do lint ./...
 → go test ./...

go do deploy
```

## Adding lint rules

Run `go do lint` to verify code standards are met and `go do lint --list` to display code standards.

To enforce standards we prefer software tools that tell you exactly what standards are not met and where. The [multichecker package](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) provides a way to build this.

Contrast this approach to documenting standards in README.md / AGENTS.md / CLAUDE.md, which leaves both developers and LLMs room to interpret and forget. A better agentic approach is to tell Claude to write code:

> Write an analysis package that enforces "use github.com/pkg/errors everywhere"
>
> ⏺ I'll explore to understand analysis packages, then create one that enforces the use of the errors packages.
> ⏺ Now I'll create the analyzer that will flag direct use of err

## Development

Run `go do dev` to live reload your `cmd/app` program.

## Deploy

Run `go do deploy` to deploy you program. It will prompt for Google Cloud settings on first run. Run `go do logs` and `go do status` to inspect deployments.


```bash
# install dependencies to manage Google Cloud
brew install gcloud-cli ko
```
