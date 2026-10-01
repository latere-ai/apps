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

The `latere-app.yaml` manifest grammar and the authorization vocabulary are
added here as packages of their own.

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
package depends on the standard library alone and clears a 90% coverage
floor.

## License

Apache License 2.0. See [LICENSE](LICENSE).
