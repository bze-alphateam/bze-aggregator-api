package converter

import "github.com/bze-alphateam/bze-aggregator-api/app/entity"

// SplitIntSlice splits a slice of ints into batches of a specified size
func SplitIntSlice(data []int, batchSize int) [][]int {
	var batches [][]int
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}
		batches = append(batches, data[i:end])
	}
	return batches
}

func SplitIntervalsSlice(data []*entity.MarketHistoryInterval, batchSize int) [][]*entity.MarketHistoryInterval {
	var batches [][]*entity.MarketHistoryInterval
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}
		batches = append(batches, data[i:end])
	}
	return batches
}
