# apps

The public Go module of Latere Apps, the platform capability that builds an
app from its repository and serves it at its own address. It holds what a
client has to agree on with the server: the CLI, an agent's skill and the
platform console import it, and none of them imports the server.

```sh
go get latere.ai/x/apps
```

## Packages

| Package | What it gives you |
|---|---|
| [`codes`](codes/) | Every error code Apps answers with: the HTTP status, the one sentence a user reads, and the one hint. A client prints the same sentence offline that the API sends |
| [`manifest`](manifest/) | The grammar of `latere-app.yaml`: `Parse` refuses a key the platform does not know with its line, `Detect` tells a static site from a service, `Resolve` applies the defaults and every rule to one tree, and `Schema` is the same grammar as a JSON Schema |
| [`authorizer`](authorizer/) | The vocabulary an authorization endpoint for Apps is written against: every action the server asks, the resource each question carries, and the limits an allow may grant, on the envelope of `latere.ai/x/pkg/authz` |

## Use

```go
import "latere.ai/x/apps/codes"

if c, ok := codes.Lookup(apiErr.Code); ok {
	fmt.Println(c.Message)
	fmt.Println(c.Hint)
}
```

## Contributing

`go tool lateregate` runs the whole quality bar, the same one CI runs. Every
package clears a 90% coverage floor. `codes` depends on the standard library
alone and `manifest` on it and `github.com/goccy/go-yaml`, whose errors carry
the line of a node, so a client imports them without a server's
dependencies; `authorizer` imports `latere.ai/x/pkg/authz`, the shared
authorization contract it is written on.

## License

Apache License 2.0. See [LICENSE](LICENSE).
