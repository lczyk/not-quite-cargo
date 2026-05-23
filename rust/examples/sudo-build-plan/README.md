# sudo-rs compilation demo (rust port)

mirrors [the go example](../../../go/examples/sudo-build-plan/). six stages:
image -> binary -> cross -> clone -> plan -> run -> prove.

note the extra `cross` stage. go cross-compiles to linux from any host with
`GOOS=linux GOARCH=<arch>` but rust is a bit harder. since we don't wanna
mess with hosts `rustup` etc, we just spin up `ubuntu/rust:1.85-24.04_edge` rock
and compile caro there to get a proper Linux ELF.

everything else is pretty miuch the same
