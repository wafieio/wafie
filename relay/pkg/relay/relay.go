package relay

import wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"

type StartRelayFunc func()
type StopRelayFunc func()

type Relay interface {
	Configure(*wv1.RelayOptions) (StartRelayFunc, StopRelayFunc)
}
