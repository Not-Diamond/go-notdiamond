// statistics.go
package notdiamond

import (
	"errors"
	"time"
)

// DataPoint represents a single data point in a time series.
type DataPoint struct {
	timestamp time.Time
	value     float64
}

// Statistics is a generic structure to hold time series data.
type Statistics struct {
	data []DataPoint
}

// NewStatistics returns a new instance of Statistics.
func newStatistics() *Statistics {
	return &Statistics{
		data: make([]DataPoint, 0),
	}
}

// Add appends a new data point to the series.
func (s *Statistics) add(ts time.Time, value float64) {
	s.data = append(s.data, DataPoint{timestamp: ts, value: value})
}

// Sum calculates the total sum of all data values.
func (s *Statistics) sum() float64 {
	var sum float64
	for _, dp := range s.data {
		sum += dp.value
	}
	return sum
}

// Average calculates the average value of the data points.
// Returns an error if there are no data points.
func (s *Statistics) average() (float64, error) {
	n := len(s.data)
	if n == 0 {
		return 0, errors.New("no data points available")
	}
	return s.sum() / float64(n), nil
}

// MovingAverage calculates the moving (or rolling) average for the data points,
// using the specified window size. The returned slice is aligned with the data slice.
func (s *Statistics) movingAverage(windowSize int) ([]float64, error) {
	if windowSize <= 0 {
		return nil, errors.New("window size must be greater than 0")
	}
	var result []float64
	n := len(s.data)
	for i := 0; i < n; i++ {
		start := i - windowSize + 1
		if start < 0 {
			start = 0
		}
		var sum float64
		for j := start; j <= i; j++ {
			sum += s.data[j].value
		}
		result = append(result, sum/float64(i-start+1))
	}
	return result, nil
}

// Min returns the minimum value in the data set.
func (s *Statistics) min() (float64, error) {
	if len(s.data) == 0 {
		return 0, errors.New("no data points available")
	}
	min := s.data[0].value
	for _, dp := range s.data {
		if dp.value < min {
			min = dp.value
		}
	}
	return min, nil
}

// Max returns the maximum value in the data set.
func (s *Statistics) max() (float64, error) {
	if len(s.data) == 0 {
		return 0, errors.New("no data points available")
	}
	max := s.data[0].value
	for _, dp := range s.data {
		if dp.value > max {
			max = dp.value
		}
	}
	return max, nil
}
