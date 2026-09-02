package sync

import "fmt"

const (
	intervalLock   = "sync:interval:%s"
	historyLock    = "sync:history:%s"
	orderLock      = "sync:order:%s"
	lpLock         = "sync:lps"
	swapEventsLock = "sync:swap_events"
)

type locker interface {
	Lock(key string)
	Unlock(key string)
}

func getIntervalLockKey(marketId string) string {
	return fmt.Sprintf(intervalLock, marketId)
}

func getHistoryLockKey(marketId string) string {
	return fmt.Sprintf(historyLock, marketId)
}

func getOrderLockKey(marketId string) string {
	return fmt.Sprintf(orderLock, marketId)
}

func getLpLockKey() string {
	return lpLock
}

func getSwapEventsLockKey() string {
	return swapEventsLock
}
