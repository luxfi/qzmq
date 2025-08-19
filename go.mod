module github.com/luxfi/qzmq

go 1.24.5

require (
	github.com/luxfi/czmq/v4 v4.2.0
	github.com/pebbe/zmq4 v1.2.10
	golang.org/x/crypto v0.40.0
)

require golang.org/x/sys v0.34.0 // indirect

replace (
	github.com/luxfi/crypto => ../crypto
	github.com/luxfi/czmq/v4 => ../czmq
	github.com/luxfi/zmq/v4 => ../zmq
)
