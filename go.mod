module github.com/luxfi/qzmq

go 1.24.5

toolchain go1.24.6

require (
	github.com/luxfi/zmq/v4 v4.2.0
	github.com/pebbe/zmq4 v1.4.0
	golang.org/x/crypto v0.40.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/luxfi/czmq/v4 v4.2.0 // indirect
	github.com/luxfi/log v1.1.22 // indirect
	github.com/luxfi/metric v1.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace (
	github.com/luxfi/crypto => ../crypto
	github.com/luxfi/zmq/v4 => ../zmq
)
