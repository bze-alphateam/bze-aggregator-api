package converter

import "fmt"

func GetMarketId(base, quote string) string {
	return fmt.Sprintf("%s/%s", base, quote)
}
