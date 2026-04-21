package contract

import "time"

type TimeoutClass string

const (
	TimeoutProbe      TimeoutClass = "probe"
	TimeoutRead       TimeoutClass = "read"
	TimeoutMutate     TimeoutClass = "mutate"
	TimeoutLongMutate TimeoutClass = "long_mutate"
	TimeoutStream     TimeoutClass = "stream"
)

type TimeoutPolicy struct {
	Class             TimeoutClass
	DefaultTimeout    time.Duration
	UsesClientTimeout bool
}

var timeoutPolicies = map[TimeoutClass]TimeoutPolicy{
	TimeoutProbe: {
		Class:             TimeoutProbe,
		DefaultTimeout:    2 * time.Second,
		UsesClientTimeout: true,
	},
	TimeoutRead: {
		Class:             TimeoutRead,
		DefaultTimeout:    15 * time.Second,
		UsesClientTimeout: true,
	},
	TimeoutMutate: {
		Class:             TimeoutMutate,
		DefaultTimeout:    30 * time.Second,
		UsesClientTimeout: true,
	},
	TimeoutLongMutate: {
		Class:             TimeoutLongMutate,
		DefaultTimeout:    120 * time.Second,
		UsesClientTimeout: true,
	},
	TimeoutStream: {
		Class:             TimeoutStream,
		DefaultTimeout:    0,
		UsesClientTimeout: false,
	},
}

func TimeoutPolicyForClass(class TimeoutClass) TimeoutPolicy {
	if policy, ok := timeoutPolicies[class]; ok {
		return policy
	}
	return timeoutPolicies[TimeoutRead]
}

func DefaultTimeout(class TimeoutClass) time.Duration {
	return TimeoutPolicyForClass(class).DefaultTimeout
}
