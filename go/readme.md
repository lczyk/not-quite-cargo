# go version of not-quite-cargo

Not-quite-car**go**...

This is an AI rewrite of the python version which was then audited for bits
quite literally lost in translation. I would not trust this version as much as
the other one. Over time though, as i vet it more thougoughly, this will become
the default verion due to the ease cross compilation + binary size + it being 
only one binary.

## layout

- `cmd/not-quite-cargo/` -- cli entrypoint, uses [go-flags](https://github.com/jessevdk/go-flags)
- `cargo/` -- library package (config, plan, patch, run, topo, deepreplace, directives)
- `cargo/testdata/` -- fixtures for the patch golden test

## build

```
make build      # ./bin/not-quite-cargo
make test       # go test -race ./...
make lint       # go vet + gofmt -l check
make fmt        # gofmt -s -w
make clean
```

`make help` lists the rest.

## usage

```
./bin/not-quite-cargo patch build_plan.json
./bin/not-quite-cargo run   build_plan.json
```