module demo

go 1.16

replace (
	github.com/binance-chain/tss-lib => gitlab.com/thorchain/tss/tss-lib v0.0.0-20201118045712-70b2cb4bf916
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
)

require (
	github.com/binance-chain/tss-lib v0.0.0-20201118045712-70b2cb4bf916
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/rs/zerolog v1.23.0
	gitlab.com/thorchain/tss/go-tss v1.3.0
)
