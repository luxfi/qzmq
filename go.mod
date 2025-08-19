module github.com/luxfi/qzmq

go 1.24.5

require (
	github.com/luxfi/zmq/v4 v4.0.0
	golang.org/x/crypto v0.40.0
)

require golang.org/x/sys v0.34.0 // indirect

replace (
	github.com/luxfi/crypto => ../crypto
	github.com/luxfi/zmq/v4 => ../zmq
)
