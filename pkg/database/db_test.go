package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// contains is a helper to check if a slice of strings contains a given item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// TestOpenNewDatabase verifies that a new Instance is created and contains the default "keystore" table.
func TestOpenNewDatabase(t *testing.T) {
	dbName := "testdb_open"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	// Check that the keystore table exists.
	cols, err := db.GetColumns("keystore")
	if err != nil {
		t.Errorf("GetColumns() failed: %v", err)
	}
	if len(cols) == 0 {
		t.Error("keystore table should have columns")
	}
	if err := db.CloseConnection(); err != nil {
		t.Errorf("CloseConnection() failed: %v", err)
	}
}

// TestMakeTables creates both a non–timeseries table and a timeseries table and verifies their column names.
func TestMakeTables(t *testing.T) {
	db, err := Open("testdb_make", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Instance) {
		err := db.CloseConnection()
		if err != nil {

		}
	}(db)

	// Create a non–timeseries table.
	nonTSTable := "non_ts"
	columns := map[string]string{
		"value": "TEXT",
	}
	if err := db.CreateTables(false, nonTSTable, columns); err != nil {
		t.Fatalf("CreateTables(non-timeseries) failed: %v", err)
	}
	cols, err := db.GetColumns(nonTSTable)
	if err != nil {
		t.Fatalf("GetColumns(non_ts) failed: %v", err)
	}
	// For non-timeseries tables, expect a primary key column "key" plus our custom columns.
	if !contains(cols, "key") {
		t.Errorf("Expected column 'key' in table %s", nonTSTable)
	}
	if !contains(cols, "value") {
		t.Errorf("Expected column 'value' in table %s", nonTSTable)
	}

	// Create a timeseries table.
	tsTable := "ts_table"
	columnsTS := map[string]string{
		"metric": "REAL",
	}
	if err := db.CreateTables(true, tsTable, columnsTS); err != nil {
		t.Fatalf("CreateTables(timeseries) failed: %v", err)
	}
	cols, err = db.GetColumns(tsTable)
	if err != nil {
		t.Fatalf("GetColumns(ts_table) failed: %v", err)
	}
	// For timeseries tables, expect "id", "timestamp" plus custom columns.
	if !contains(cols, "id") {
		t.Errorf("Expected column 'id' in timeseries table")
	}
	if !contains(cols, "timestamp") {
		t.Errorf("Expected column 'timestamp' in timeseries table")
	}
	if !contains(cols, "metric") {
		t.Errorf("Expected column 'metric' in timeseries table")
	}
}

// TestSetGetDelete exercises the SetJSON, GetJSON, and deleteItem operations.
func TestSetGetDelete(t *testing.T) {
	db, err := Open("testdb_setget", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Instance) {
		err := db.CloseConnection()
		if err != nil {

		}
	}(db)

	tableName := "keystore"
	key := "testKey"
	value := "testValue"

	// Test SetJSON.
	if err := db.SetJSON(tableName, key, value); err != nil {
		t.Fatalf("SetJSON() failed: %v", err)
	}

	// Test GetJSON.
	var got string
	if err := db.GetJSON(tableName, key, &got); err != nil {
		t.Fatalf("GetJSON() failed: %v", err)
	}
	if got != value {
		t.Errorf("GetJSON() returned %v; want %v", got, value)
	}

	// Test deleteItem.
	if err := db.deleteItem(tableName, key); err != nil {
		t.Fatalf("deleteItem() failed: %v", err)
	}
	// After deletion, GetJSON should return an error.
	if err := db.GetJSON(tableName, key, &got); err == nil {
		t.Errorf("Expected error after deleteItem, got nil")
	}
}

// TestDump verifies that dumpDB returns an SQL dump containing known table names.
func TestDump(t *testing.T) {
	db, err := Open("testdb_dump", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Instance) {
		err := db.CloseConnection()
		if err != nil {

		}
	}(db)

	// Insert a value into keystore.
	key := "dumpKey"
	value := "dumpValue"
	if err := db.SetJSON("keystore", key, value); err != nil {
		t.Fatalf("SetJSON() failed: %v", err)
	}

	dump, err := db.dumpDB()
	if err != nil {
		t.Fatalf("dumpDB() failed: %v", err)
	}
	if !strings.Contains(dump, "keystore") {
		t.Errorf("dumpDB() does not contain 'keystore' table definition")
	}
}

// TestExists verifies the entryExists function for both an existing and a non-existing Instance.
func TestExists(t *testing.T) {
	dbName := "existsdb"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	err = db.CloseConnection()
	if err != nil {
		t.Errorf("CloseConnection() failed: %v", err)
	}

	// entryExists should succeed.
	if err := entryExists(dbName); err != nil {
		t.Errorf("entryExists() returned error for existing DB: %v", err)
	}

	// For a non-existent Instance, entryExists should return an error.
	nonExisting := "nonexistentdb"
	if err := entryExists(nonExisting); err == nil {
		t.Errorf("entryExists() expected error for non-existing DB")
	}
}

// TestDrop ensures that Drop closes the DB, releases the lock, and removes the file.
func TestDrop(t *testing.T) {
	dbName := "dropdb"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	dbFile := db.Schema

	if err := db.Drop(); err != nil {
		t.Fatalf("Drop() failed: %v", err)
	}
	// Verify that the file no longer exists.
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("Instance file still exists after Drop()")
	}
}

// TestQueryExec tests Exec and Query by creating a temporary table, inserting a row, and querying it.
func TestQueryExec(t *testing.T) {
	db, err := Open("testdb_queryexec", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Instance) {
		err := db.CloseConnection()
		if err != nil {

		}
	}(db)

	createStmt := `CREATE TABLE IF NOT EXISTS temp_table (id INTEGER PRIMARY KEY, name TEXT);`
	if err := db.Exec(createStmt); err != nil {
		t.Fatalf("Exec(create table) failed: %v", err)
	}

	insertStmt := `INSERT INTO temp_table (name) VALUES (?);`
	if err := db.Exec(insertStmt, "Alice"); err != nil {
		t.Fatalf("Exec(insert) failed: %v", err)
	}

	rows, err := db.Query("SELECT name FROM temp_table WHERE name = ?", "Alice")
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var name string
	if rows.Next() {
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Row Scan failed: %v", err)
		}
		if name != "Alice" {
			t.Errorf("Expected 'Alice', got %v", name)
		}
	} else {
		t.Errorf("No rows returned from Query()")
	}
}

// TestInvalidQuery ensures that Exec returns an error for an invalid SQL statement.
func TestInvalidQuery(t *testing.T) {
	db, err := Open("testdb_invalidquery", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Instance) {
		err := db.CloseConnection()
		if err != nil {

		}
	}(db)

	err = db.Exec("INVALID SQL STATEMENT")
	if err == nil {
		t.Errorf("Expected error for invalid SQL statement, got nil")
	}
}

// TestOpen verifies database creation, opening, and locking behavior.
func TestOpen(t *testing.T) {
	// Use a temporary directory for test isolation
	tmpDir := t.TempDir()
	originalDataFolder := DataFolder
	DataFolder = tmpDir
	defer func() {
		DataFolder = originalDataFolder
	}()

	tests := []struct {
		name      string
		dbName    string
		readOnly  bool
		wantErr   bool
		setup     func(t *testing.T) (*Instance, error)
		cleanup   func(*Instance)
		errString string
	}{
		{
			name:     "new database creation",
			dbName:   "test_new.db",
			readOnly: false,
			wantErr:  false,
		},
		{
			name:     "concurrent access attempt",
			dbName:   "concurrent.db",
			readOnly: false,
			setup: func(t *testing.T) (*Instance, error) {
				return Open("concurrent.db", false)
			},
			wantErr:   true,
			errString: "database is locked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var setupDB *Instance

			// Setup if needed
			if tt.setup != nil {
				var err error
				setupDB, err = tt.setup(t)
				if err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			// Ensure cleanup happens
			if tt.cleanup != nil {
				defer tt.cleanup(setupDB)
			} else if setupDB != nil {
				defer setupDB.CloseConnection()
			}

			// Create a channel for the test operation
			done := make(chan struct{})
			var db *Instance
			var err error

			// Run the Open operation in a goroutine
			go func() {
				db, err = Open(tt.dbName, tt.readOnly)
				close(done)
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				if (err != nil) != tt.wantErr {
					t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				}
				if err != nil && tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Open() error = %v, want error containing %v", err, tt.errString)
				}
			case <-time.After(1 * time.Second):
				if !tt.wantErr {
					t.Error("Open() timed out")
				}
			}

			// Cleanup the test database
			if db != nil {
				if err := db.CloseConnection(); err != nil {
					t.Errorf("Failed to close database: %v", err)
				}
			}
		})
	}
}

func TestCreateTables(t *testing.T) {
	tests := []struct {
		name       string
		timeSeries bool
		tableName  string
		columns    map[string]string
		wantErr    bool
		validate   func(*testing.T, *Instance, string)
	}{
		{
			name:       "create timeseries table",
			timeSeries: true,
			tableName:  "test_timeseries",
			columns: map[string]string{
				"value": "REAL",
				"tag":   "TEXT",
			},
			wantErr: false,
			validate: func(t *testing.T, d *Instance, tableName string) {
				// Verify columns exist
				cols, err := d.GetColumns(tableName)
				if err != nil {
					t.Errorf("Failed to get columns: %v", err)
					return
				}
				expected := []string{"id", "timestamp", "value", "tag"}
				if len(cols) != len(expected) {
					t.Errorf("Expected %v columns, got %v", expected, cols)
				}
			},
		},
		{
			name:       "create key-value table",
			timeSeries: false,
			tableName:  "test_keyvalue",
			columns: map[string]string{
				"value": "TEXT",
			},
			wantErr: false,
			validate: func(t *testing.T, d *Instance, tableName string) {
				// Verify columns exist
				cols, err := d.GetColumns(tableName)
				if err != nil {
					t.Errorf("Failed to get columns: %v", err)
					return
				}
				expected := []string{"key", "value"}
				if len(cols) != len(expected) {
					t.Errorf("Expected %v columns, got %v", expected, cols)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temporary directory
			tmpDir := t.TempDir()
			DataFolder = tmpDir

			db, err := Open("test_db", false)
			if err != nil {
				t.Fatalf("Failed to open database: %v", err)
			}
			defer db.CloseConnection()

			if err := db.CreateTables(tt.timeSeries, tt.tableName, tt.columns); (err != nil) != tt.wantErr {
				t.Errorf("CreateTables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, db, tt.tableName)
			}
		})
	}
}

func TestSetAndGetJSON(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name     string
		key      string
		value    testStruct
		wantErr  bool
		validate func(*testing.T, *Instance, string, testStruct)
	}{
		{
			name: "store and retrieve json",
			key:  "test_key",
			value: testStruct{
				Name:  "test",
				Value: 42,
			},
			wantErr: false,
			validate: func(t *testing.T, d *Instance, key string, expected testStruct) {
				var retrieved testStruct
				err := d.GetJSON("keystore", key, &retrieved)
				if err != nil {
					t.Errorf("Failed to retrieve JSON: %v", err)
					return
				}
				if retrieved != expected {
					t.Errorf("Retrieved value = %v, want %v", retrieved, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			DataFolder = tmpDir

			db, err := Open("test_db", false)
			if err != nil {
				t.Fatalf("Failed to open database: %v", err)
			}
			defer db.CloseConnection()

			if err := db.SetJSON("keystore", tt.key, tt.value); (err != nil) != tt.wantErr {
				t.Errorf("SetJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, db, tt.key, tt.value)
			}
		})
	}
}

func TestDeleteItem(t *testing.T) {
	// Use a temporary directory for test isolation
	tmpDir := t.TempDir()
	originalDataFolder := DataFolder
	DataFolder = tmpDir
	defer func() {
		DataFolder = originalDataFolder
	}()

	tests := []struct {
		name      string
		tableName string
		key       string
		setup     func(*Instance) error
		validate  func(*testing.T, *Instance)
		wantErr   bool
	}{
		{
			name:      "delete existing item",
			tableName: "keystore",
			key:       "test_key",
			setup: func(db *Instance) error {
				return db.SetJSON("keystore", "test_key", "test_value")
			},
			validate: func(t *testing.T, db *Instance) {
				var value string
				err := db.GetJSON("keystore", "test_key", &value)
				if err == nil {
					t.Error("Expected error getting deleted key, got nil")
				}
			},
			wantErr: false,
		},
		{
			name:      "delete nonexistent item",
			tableName: "keystore",
			key:       "nonexistent_key",
			wantErr:   false,
		},
		{
			name:      "invalid table name",
			tableName: "nonexistent_table",
			key:       "test_key",
			setup: func(db *Instance) error {
				// First verify the table doesn't exist
				rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name=?", "nonexistent_table")
				if err != nil {
					return err
				}
				defer rows.Close()
				if rows.Next() {
					return fmt.Errorf("table should not exist")
				}
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new database for each test
			db, err := Open("test_delete_"+tt.name, false)
			if err != nil {
				t.Fatalf("Failed to open database: %v", err)
			}
			defer func() {
				if err := db.CloseConnection(); err != nil {
					t.Errorf("Failed to close database: %v", err)
				}
			}()

			// Run setup if provided
			if tt.setup != nil {
				if err := tt.setup(db); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			// Execute deleteItem
			err = db.deleteItem(tt.tableName, tt.key)

			// Check error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("deleteItem() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Run validation if provided
			if tt.validate != nil {
				tt.validate(t, db)
			}
		})
	}
}
