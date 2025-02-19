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
