module github.com/luxfi/qzmq

go 1.24.5

toolchain go1.24.6

require (
	github.com/pebbe/zmq4 v1.2.10
	golang.org/x/crypto v0.40.0
)

require golang.org/x/sys v0.34.0 // indirect

replace github.com/luxfi/crypto => ../crypto
