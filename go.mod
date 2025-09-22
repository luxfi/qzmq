module github.com/luxfi/qzmq

go 1.24.5

toolchain go1.24.6

require (
	github.com/luxfi/crypto v0.0.0-00010101000000-000000000000
	github.com/luxfi/zmq/v4 v4.2.0
	golang.org/x/crypto v0.41.0
)

require (
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/luxfi/czmq/v4 v4.2.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)

replace (
	github.com/luxfi/crypto => ../crypto
	github.com/luxfi/zmq/v4 => ../zmq
)
