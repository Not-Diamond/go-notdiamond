package statistic

import (
	"errors"
	"time"
)

// DataPoint represents a single data point in a time series.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// Statistic is a generic structure to hold time series data.
type Statistic struct {
	Data []DataPoint
}

// Add appends a new data point to the series.
func (s *Statistic) Add(ts time.Time, value float64) {
	s.Data = append(s.Data, DataPoint{Timestamp: ts, Value: value})
}

// Sum calculates the total sum of all data values.
func (s *Statistic) sum() float64 {
	var sum float64
	for _, dp := range s.Data {
		sum += dp.Value
	}
	return sum
}

// Average calculates the average value of the data points.
// Returns an error if there are no data points.
func (s *Statistic) average() (float64, error) {
	n := len(s.Data)
	if n == 0 {
		return 0, errors.New("no data points available")
	}
	return s.sum() / float64(n), nil
}

// MovingAverage calculates the moving (or rolling) average for the data points,
// using the specified window size. The returned slice is aligned with the data slice.
func (s *Statistic) MovingAverage(windowSize int) ([]float64, error) {
	if windowSize <= 0 {
		return nil, errors.New("window size must be greater than 0")
	}
	var result []float64
	n := len(s.Data)
	for i := 0; i < n; i++ {
		start := i - windowSize + 1
		if start < 0 {
			start = 0
		}
		var sum float64
		for j := start; j <= i; j++ {
			sum += s.Data[j].Value
		}
		result = append(result, sum/float64(i-start+1))
	}
	return result, nil
}

// Min returns the minimum value in the data set.
func (s *Statistic) Min() (float64, error) {
	if len(s.Data) == 0 {
		return 0, errors.New("no data points available")
	}
	min := s.Data[0].Value
	for _, dp := range s.Data {
		if dp.Value < min {
			min = dp.Value
		}
	}
	return min, nil
}

// Max returns the maximum value in the data set.
func (s *Statistic) Max() (float64, error) {
	if len(s.Data) == 0 {
		return 0, errors.New("no data points available")
	}
	max := s.Data[0].Value
	for _, dp := range s.Data {
		if dp.Value > max {
			max = dp.Value
		}
	}
	return max, nil
}
