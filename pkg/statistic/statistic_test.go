package statistic

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// Helper function to compare float64 values with a tolerance.
func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func TestAddAndSum(t *testing.T) {
	stats := &Statistic{
		Data: []DataPoint{},
	}
	stats.Add(time.Now(), 1.0)
	stats.Add(time.Now(), 2.0)
	stats.Add(time.Now(), 3.0)

	expectedSum := 6.0
	sum := stats.sum()
	if !almostEqual(sum, expectedSum, 1e-9) {
		t.Errorf("Expected sum %f, got %f", expectedSum, sum)
	}
}

func TestAverageEmpty(t *testing.T) {
	stats := &Statistic{
		Data: []DataPoint{},
	}
	_, err := stats.average()
	if err == nil {
		t.Error("Expected error for Average on empty data, got nil")
	}
}

func TestAverageNonEmpty(t *testing.T) {
	stats := &Statistic{
		Data: []DataPoint{},
	}
	stats.Add(time.Now(), 1.0)
	stats.Add(time.Now(), 2.0)
	stats.Add(time.Now(), 3.0)
	avg, err := stats.average()
	if err != nil {
		t.Fatalf("Unexpected error computing Average: %v", err)
	}
	expectedAvg := 2.0
	if !almostEqual(avg, expectedAvg, 1e-9) {
		t.Errorf("Expected average %f, got %f", expectedAvg, avg)
	}
}

func TestMovingAverage_WindowSizeOne(t *testing.T) {
	// With window size 1, each moving average value should equal the original value.
	stats := &Statistic{
		Data: []DataPoint{},
	}
	baseTime := time.Now()
	values := []float64{1, 2, 3, 4, 5}
	for i, v := range values {
		stats.Add(baseTime.Add(time.Duration(i)*time.Second), v)
	}

	mavg, err := stats.MovingAverage(1)
	if err != nil {
		t.Fatalf("Unexpected error computing MovingAverage: %v", err)
	}
	if len(mavg) != len(values) {
		t.Fatalf("Expected moving average length %d, got %d", len(values), len(mavg))
	}
	for i, v := range values {
		if !almostEqual(mavg[i], v, 1e-9) {
			t.Errorf("At index %d, expected moving average %f, got %f", i, v, mavg[i])
		}
	}
}

func TestMovingAverage_WindowSizeThree(t *testing.T) {
	// Using window size 3 on the data [1, 2, 3, 4, 5]:
	// index 0: average(1) = 1
	// index 1: average(1,2) = 1.5
	// index 2: average(1,2,3) = 2.0
	// index 3: average(2,3,4) = 3.0
	// index 4: average(3,4,5) = 4.0
	stats := &Statistic{
		Data: []DataPoint{},
	}
	baseTime := time.Now()
	values := []float64{1, 2, 3, 4, 5}
	for i, v := range values {
		stats.Add(baseTime.Add(time.Duration(i)*time.Second), v)
	}

	mavg, err := stats.MovingAverage(3)
	if err != nil {
		t.Fatalf("Unexpected error computing MovingAverage: %v", err)
	}
	expected := []float64{1, 1.5, 2.0, 3.0, 4.0}
	if len(mavg) != len(expected) {
		t.Fatalf("Expected moving average length %d, got %d", len(expected), len(mavg))
	}
	for i, expVal := range expected {
		if !almostEqual(mavg[i], expVal, 1e-9) {
			t.Errorf("At index %d, expected %f, got %f", i, expVal, mavg[i])
		}
	}
}

func TestMovingAverage_WindowSizeLargerThanData(t *testing.T) {
	// When window size exceeds the number of data points, the average is computed on the available points.
	stats := &Statistic{
		Data: []DataPoint{},
	}
	baseTime := time.Now()
	values := []float64{10, 20}
	for i, v := range values {
		stats.Add(baseTime.Add(time.Duration(i)*time.Second), v)
	}

	mavg, err := stats.MovingAverage(5)
	if err != nil {
		t.Fatalf("Unexpected error computing MovingAverage: %v", err)
	}
	// Expected: index0: average(10)=10, index1: average(10,20)=15
	expected := []float64{10, 15}
	if !reflect.DeepEqual(mavg, expected) {
		t.Errorf("Expected moving averages %v, got %v", expected, mavg)
	}
}

func TestMovingAverage_InvalidWindow(t *testing.T) {
	stats := &Statistic{
		Data: []DataPoint{},
	}
	stats.Add(time.Now(), 1.0)
	_, err := stats.MovingAverage(0)
	if err == nil {
		t.Error("Expected error for window size <= 0, got nil")
	}
}

func TestMinAndMax(t *testing.T) {
	stats := &Statistic{
		Data: []DataPoint{},
	}
	// Test on empty data: expect error.
	if _, err := stats.Min(); err == nil {
		t.Error("Expected error for Min on empty data, got nil")
	}
	if _, err := stats.Max(); err == nil {
		t.Error("Expected error for Max on empty data, got nil")
	}

	// Add multiple data points.
	stats.Add(time.Now(), 5.0)
	stats.Add(time.Now(), 2.0)
	stats.Add(time.Now(), 8.0)
	stats.Add(time.Now(), 3.0)

	min, err := stats.Min()
	if err != nil {
		t.Fatalf("Unexpected error computing Min: %v", err)
	}
	if !almostEqual(min, 2.0, 1e-9) {
		t.Errorf("Expected Min to be 2.0, got %f", min)
	}

	max, err := stats.Max()
	if err != nil {
		t.Fatalf("Unexpected error computing Max: %v", err)
	}
	if !almostEqual(max, 8.0, 1e-9) {
		t.Errorf("Expected Max to be 8.0, got %f", max)
	}
}
