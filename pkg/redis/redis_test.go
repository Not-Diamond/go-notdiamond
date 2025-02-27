package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}

	client, err := NewClient(Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		mr.Close()
		t.Fatalf("Failed to create Redis client: %v", err)
	}

	return client, mr
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid configuration",
			cfg: Config{
				Addr: "localhost:6379",
			},
			wantErr: false,
		},
		{
			name: "invalid address",
			cfg: Config{
				Addr: "invalid:address",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "valid configuration" {
				// Use miniredis for the valid case
				mr, err := miniredis.Run()
				if err != nil {
					t.Fatalf("Failed to create miniredis: %v", err)
				}
				defer mr.Close()
				tt.cfg.Addr = mr.Addr()
			}

			client, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if err := client.Close(); err != nil {
					t.Errorf("Failed to close client: %v", err)
				}
			}
		})
	}
}

func TestRecordLatency(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"
	latency := 0.5
	status := "success"

	err := client.RecordLatency(ctx, model, latency, status)
	if err != nil {
		t.Fatalf("RecordLatency() error = %v", err)
	}

	// Verify data was stored
	key := "latency:test-model"
	if !mr.Exists(key) {
		t.Error("Expected latency key to exist in Redis")
	}

	counterKey := "latency:test-model:counter"
	count, err := mr.Get(counterKey)
	if err != nil {
		t.Errorf("Failed to get counter: %v", err)
	}
	if count != "1" {
		t.Errorf("Expected counter to be 1, got %s", count)
	}
}

func TestGetAverageLatency(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Record some test latencies
	latencies := []float64{0.5, 1.0, 1.5}
	for _, l := range latencies {
		err := client.RecordLatency(ctx, model, l, "success")
		if err != nil {
			t.Fatalf("RecordLatency() error = %v", err)
		}
	}

	// Test getting average of last 2 entries
	avg, err := client.GetAverageLatency(ctx, model, 2)
	if err != nil {
		t.Fatalf("GetAverageLatency() error = %v", err)
	}

	expectedAvg := (1.0 + 1.5) / 2
	if avg != expectedAvg {
		t.Errorf("GetAverageLatency() = %v, want %v", avg, expectedAvg)
	}
}

func TestSetAndCheckRecoveryTime(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"
	duration := 100 * time.Millisecond

	// Set recovery time
	err := client.SetRecoveryTime(ctx, model, duration)
	if err != nil {
		t.Fatalf("SetRecoveryTime() error = %v", err)
	}

	// Check recovery time exists
	inRecovery, err := client.CheckRecoveryTime(ctx, model)
	if err != nil {
		t.Fatalf("CheckRecoveryTime() error = %v", err)
	}
	if !inRecovery {
		t.Error("Expected model to be in recovery")
	}

	// Wait for recovery time to expire and fast-forward miniredis time
	time.Sleep(duration + 10*time.Millisecond)
	mr.FastForward(duration + 10*time.Millisecond)

	// Check recovery time has expired
	inRecovery, err = client.CheckRecoveryTime(ctx, model)
	if err != nil {
		t.Fatalf("CheckRecoveryTime() error = %v", err)
	}
	if inRecovery {
		t.Error("Expected model to not be in recovery")
	}
}

func TestCleanupOldLatencies(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Record some test latencies
	err := client.RecordLatency(ctx, model, 1.0, "success")
	if err != nil {
		t.Fatalf("RecordLatency() error = %v", err)
	}

	// Clean up latencies older than 1 second
	err = client.CleanupOldLatencies(ctx, model, time.Second)
	if err != nil {
		t.Fatalf("CleanupOldLatencies() error = %v", err)
	}

	// Verify data still exists (should be less than 1 second old)
	entries, err := client.GetLatencyEntries(ctx, model, 1)
	if err != nil {
		t.Fatalf("GetLatencyEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry after cleanup, got %d", len(entries))
	}
}

func TestGetLatencyEntries(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Test empty case first
	entries, err := client.GetLatencyEntries(ctx, model, 5)
	if err != nil {
		t.Fatalf("GetLatencyEntries() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for empty case, got %d", len(entries))
	}

	// Record some test latencies
	testLatencies := []float64{0.5, 1.0, 1.5, 2.0, 2.5}
	for _, l := range testLatencies {
		err := client.RecordLatency(ctx, model, l, "success")
		if err != nil {
			t.Fatalf("RecordLatency() error = %v", err)
		}
	}

	// Test getting all entries
	entries, err = client.GetLatencyEntries(ctx, model, 5)
	if err != nil {
		t.Fatalf("GetLatencyEntries() error = %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}

	// Test limiting entries
	entries, err = client.GetLatencyEntries(ctx, model, 3)
	if err != nil {
		t.Fatalf("GetLatencyEntries() error = %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestCleanupOldLatenciesWithData(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Record multiple latencies with old timestamps
	oldTime := time.Now().Add(-2 * time.Hour)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("latency:%s", model)
		entry := map[string]interface{}{
			"timestamp": oldTime,
			"latency":   float64(i),
			"status":    "success",
		}
		data, _ := json.Marshal(entry)
		score := float64(oldTime.Unix())
		if err := client.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: string(data)}).Err(); err != nil {
			t.Fatalf("Failed to add old entry: %v", err)
		}
		oldTime = oldTime.Add(time.Minute)
	}

	// Add one recent entry
	err := client.RecordLatency(ctx, model, 5.0, "success")
	if err != nil {
		t.Fatalf("RecordLatency() error = %v", err)
	}

	// Clean up entries older than 1 hour
	err = client.CleanupOldLatencies(ctx, model, time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldLatencies() error = %v", err)
	}

	// Verify only recent entry remains
	entries, err := client.GetLatencyEntries(ctx, model, 10)
	if err != nil {
		t.Fatalf("GetLatencyEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry after cleanup, got %d", len(entries))
	}
}

func TestRecordLatencyInvalidJSON(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Test with invalid timestamp
	err := client.RecordLatency(ctx, model, math.Inf(1), "success")
	if err == nil {
		t.Error("Expected error for invalid JSON data, got nil")
	}
}
func TestRecordLatencyRedisErrors(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	model := "test-model"
	latency := 0.5
	status := "success"

	// Test Redis error
	mr.SetError("simulated Redis error")
	err := client.RecordLatency(ctx, model, latency, status)
	if err == nil || !strings.Contains(err.Error(), "simulated Redis error") {
		t.Errorf("Expected Redis error, got: %v", err)
	}

	// Clear error and verify successful operation
	mr.SetError("")
	err = client.RecordLatency(ctx, model, latency, status)
	if err != nil {
		t.Errorf("Expected successful operation, got error: %v", err)
	}
}

func TestErrorTracking(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	modelName := "test-model"

	// Test recording error codes
	for i := 0; i < 5; i++ {
		if err := client.RecordErrorCode(ctx, modelName, 401); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Test getting error percentages
	percentages, err := client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if percentage, exists := percentages[401]; !exists || percentage != 100 {
		t.Errorf("Expected 100%% 401 errors, got %.2f%%", percentage)
	}

	// Test setting error recovery time
	recoveryTime := 1 * time.Minute
	if err := client.SetErrorRecoveryTime(ctx, modelName, recoveryTime); err != nil {
		t.Errorf("SetErrorRecoveryTime() error = %v", err)
	}

	// Test checking error recovery time
	inRecovery, err := client.CheckErrorRecoveryTime(ctx, modelName)
	if err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}
	if !inRecovery {
		t.Error("Expected model to be in recovery")
	}

	// Test error percentages after recovery time is set
	percentages, err = client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if len(percentages) != 0 {
		t.Error("Expected no error percentages during recovery period")
	}

	// Fast forward past recovery time
	mr.FastForward(2 * time.Minute)

	// Test checking error recovery time after expiration
	inRecovery, err = client.CheckErrorRecoveryTime(ctx, modelName)
	if err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}
	if inRecovery {
		t.Error("Expected model to be out of recovery")
	}
}

func TestErrorTrackingWithMixedStatusCodes(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	modelName := "test-model"

	// Record mixed status codes
	statusCodes := []int{401, 401, 401, 500, 500}
	for _, code := range statusCodes {
		if err := client.RecordErrorCode(ctx, modelName, code); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Test getting error percentages
	percentages, err := client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}

	expected401Percentage := 60.0 // 3 out of 5
	expected500Percentage := 40.0 // 2 out of 5

	if percentage, exists := percentages[401]; !exists || percentage != expected401Percentage {
		t.Errorf("Expected %.2f%% 401 errors, got %.2f%%", expected401Percentage, percentage)
	}
	if percentage, exists := percentages[500]; !exists || percentage != expected500Percentage {
		t.Errorf("Expected %.2f%% 500 errors, got %.2f%%", expected500Percentage, percentage)
	}
}

func TestErrorTrackingCleanup(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	modelName := "test-model"

	// Record errors with old timestamps
	oldTime := time.Now().Add(-25 * time.Hour)
	key := fmt.Sprintf("errors:%s", modelName)
	counterKey := fmt.Sprintf("errors:%s:counter", modelName)

	for i := 0; i < 5; i++ {
		entry := map[string]interface{}{
			"timestamp":   oldTime.Format(time.RFC3339),
			"status_code": 401,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("Failed to marshal entry: %v", err)
		}

		score := float64(oldTime.Unix())
		if err := client.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: string(data)}).Err(); err != nil {
			t.Fatalf("Failed to add old entry: %v", err)
		}
		oldTime = oldTime.Add(time.Minute)
	}

	// Set the counter
	if err := client.rdb.Set(ctx, counterKey, "5", 0).Err(); err != nil {
		t.Fatalf("Failed to set counter: %v", err)
	}

	// Verify errors are present before cleanup
	percentages, err := client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if len(percentages) == 0 {
		t.Error("Expected error percentages before cleanup")
	}

	// Clean up old errors
	if err := client.CleanupOldErrors(ctx, modelName, 24*time.Hour); err != nil {
		t.Errorf("CleanupOldErrors() error = %v", err)
	}

	// Verify errors were cleaned up
	percentages, err = client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if len(percentages) != 0 {
		t.Error("Expected no error percentages after cleanup")
	}

	// Verify counter was reset
	count, err := client.rdb.Get(ctx, counterKey).Int64()
	if err != nil && err != redis.Nil {
		t.Errorf("Failed to get counter: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected counter to be reset to 0, got %d", count)
	}
}

func TestErrorTrackingEdgeCases(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	modelName := "test-model"

	// Test getting error percentages for non-existent model
	percentages, err := client.GetErrorPercentages(ctx, modelName, 5)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if len(percentages) != 0 {
		t.Error("Expected empty percentages for non-existent model")
	}

	// Test checking recovery time for non-existent model
	inRecovery, err := client.CheckErrorRecoveryTime(ctx, modelName)
	if err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}
	if inRecovery {
		t.Error("Expected non-existent model to not be in recovery")
	}

	// Test setting invalid recovery time
	if err := client.SetErrorRecoveryTime(ctx, modelName, -1*time.Minute); err != nil {
		t.Errorf("SetErrorRecoveryTime() with negative duration error = %v", err)
	}

	// Test getting error percentages with invalid count
	_, err = client.GetErrorPercentages(ctx, modelName, -1)
	if err == nil {
		t.Error("Expected error when getting percentages with negative count")
	}

	// Test getting error percentages with zero count
	percentages, err = client.GetErrorPercentages(ctx, modelName, 0)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if len(percentages) != 0 {
		t.Error("Expected empty percentages for zero count")
	}
}

func TestErrorTrackingRecoveryTimeExpiration(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	modelName := "test-model"

	// Set recovery time
	recoveryTime := 1 * time.Minute
	if err := client.SetErrorRecoveryTime(ctx, modelName, recoveryTime); err != nil {
		t.Errorf("SetErrorRecoveryTime() error = %v", err)
	}

	// Record some errors
	for i := 0; i < 5; i++ {
		if err := client.RecordErrorCode(ctx, modelName, 401); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Fast forward just before expiration
	mr.FastForward(59 * time.Second)

	// Should still be in recovery
	inRecovery, err := client.CheckErrorRecoveryTime(ctx, modelName)
	if err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}
	if !inRecovery {
		t.Error("Expected model to still be in recovery")
	}

	// Fast forward past expiration
	mr.FastForward(2 * time.Second)

	// Should be out of recovery
	inRecovery, err = client.CheckErrorRecoveryTime(ctx, modelName)
	if err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}
	if inRecovery {
		t.Error("Expected model to be out of recovery")
	}

	// New errors should be counted
	if err := client.RecordErrorCode(ctx, modelName, 401); err != nil {
		t.Errorf("RecordErrorCode() error = %v", err)
	}

	percentages, err := client.GetErrorPercentages(ctx, modelName, 1)
	if err != nil {
		t.Errorf("GetErrorPercentages() error = %v", err)
	}
	if percentage, exists := percentages[401]; !exists || percentage != 100 {
		t.Errorf("Expected 100%% 401 errors after recovery, got %.2f%%", percentage)
	}
}

func TestClearAllModelData(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{
		Addr:     mr.Addr(),
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Fatalf("Failed to create Redis client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	model := "test-model"

	// Set up various keys for the model
	keys := []string{
		fmt.Sprintf("latency:%s:recovery", model),
		fmt.Sprintf("errors:%s:recovery", model),
		fmt.Sprintf("latency:%s", model),
		fmt.Sprintf("latency:%s:counter", model),
		fmt.Sprintf("errors:%s", model),
		fmt.Sprintf("errors:%s:counter", model),
	}

	// Set values in Redis for each key
	for _, key := range keys {
		mr.Set(key, "test-value")
	}

	// Verify keys exist
	for _, key := range keys {
		if !mr.Exists(key) {
			t.Errorf("Key %s does not exist before test", key)
		}
	}

	// Call the function to clear all keys
	err = client.ClearAllModelData(ctx, model)
	if err != nil {
		t.Errorf("ClearAllModelData() error = %v", err)
	}

	// Verify keys no longer exist
	for _, key := range keys {
		if mr.Exists(key) {
			t.Errorf("Key %s still exists after ClearAllModelData", key)
		}
	}
}
