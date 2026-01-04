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

## Configuring deploy
