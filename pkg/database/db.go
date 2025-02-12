package database

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mr-tron/base58/base58"
	"github.com/pkg/errors"
	"github.com/schollz/sqlite3dump"
)

// dataFolder is the directory where all SQLite3 databases are stored.
// It should be set to an absolute path in production.
var DataFolder = "."

// Instance represents a SQLite Instance instance.
type Instance struct {
	Schema   string  // Full file path to the SQLite Instance.
	db       *sql.DB // Underlying Instance connection.
	isClosed bool
}

// databaseLock is used to prevent concurrent access to the same Instance file.
type databaseLock struct {
	Locked map[string]bool
	sync.RWMutex
}

var dbLock *databaseLock

func init() {
	// Initialize the Instance lock map.
	dbLock = &databaseLock{
		Locked: make(map[string]bool),
	}
}

// CreateTables creates a table with the given name and columns.
// If timeSeries is true, an auto-increment primary key and a timestamp column are added.
func (d *Instance) CreateTables(timeSeries bool, tableName string, columns map[string]string) error {
	var sqlStmt string
	if timeSeries {
		sqlStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME`, tableName)
	} else {
		sqlStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			key TEXT NOT NULL PRIMARY KEY`, tableName)
	}
	for col, colType := range columns {
		sqlStmt += fmt.Sprintf(", %s %s", col, colType)
	}
	sqlStmt += ");"

	if _, err := d.db.Exec(sqlStmt); err != nil {
		err = errors.Wrap(err, "CreateTables Exec")
		slog.Error(err.Error())
		return err
	}

	// Create an index on the primary lookup column.
	if timeSeries {
		sqlStmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_idx ON %s(timestamp);`, tableName, tableName)
	} else {
		sqlStmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_idx ON %s(key);`, tableName, tableName)
	}
	if _, err := d.db.Exec(sqlStmt); err != nil {
		err = errors.Wrap(err, "CreateTables Index")
		slog.Error(err.Error())
		return err
	}

	return nil
}

// GetColumns returns the list of column names for the specified table.
func (d *Instance) GetColumns(table string) ([]string, error) {
	rows, err := d.db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 1", table))
	if err != nil {
		return nil, errors.Wrap(err, "GetColumns Query")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, errors.Wrap(err, "GetColumns Scan")
	}
	return cols, nil
}

// GetJSON retrieves the JSON value associated with the specified key from the table,
// unmarshalling it into the provided interface.
func (d *Instance) GetJSON(tableName, key string, v interface{}) error {
	query := fmt.Sprintf("SELECT value FROM %s WHERE key = ?", tableName)
	stmt, err := d.db.Prepare(query)
	if err != nil {
		return errors.Wrap(err, "GetJSON Prepare")
	}
	defer stmt.Close()

	var result string
	if err := stmt.QueryRow(key).Scan(&result); err != nil {
		return errors.Wrap(err, "GetJSON QueryRow")
	}

	if err := json.Unmarshal([]byte(result), v); err != nil {
		return errors.Wrap(err, "GetJSON Unmarshal")
	}
	return nil
}

// SetJSON inserts or updates the given key-value pair in the specified table.
// The value is stored as a JSON string.
func (d *Instance) SetJSON(tableName, key string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "SetJSON Marshal")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return errors.Wrap(err, "SetJSON Begin")
	}
	stmt, err := tx.Prepare(fmt.Sprintf("INSERT OR REPLACE INTO %s(key, value) VALUES (?, ?)", tableName))
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "SetJSON Prepare")
	}
	defer stmt.Close()

	if _, err := stmt.Exec(key, string(b)); err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "SetJSON Exec")
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "SetJSON Commit")
	}
	return nil
}

// deleteItem removes the record identified by key from the specified table.
func (d *Instance) deleteItem(tableName, key string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return errors.Wrap(err, "deleteItem Begin")
	}
	stmt, err := tx.Prepare(fmt.Sprintf("DELETE FROM %s WHERE key = ?", tableName))
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "deleteItem Prepare")
	}
	defer stmt.Close()

	if _, err := stmt.Exec(key); err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "deleteItem Exec")
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "deleteItem Commit")
	}
	return nil
}

// dumpDB outputs a complete SQL dump of the Instance.
func (d *Instance) dumpDB() (string, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if err := sqlite3dump.Dump(d.Schema, writer); err != nil {
		return "", errors.Wrap(err, "dumpDB sqlite3dump")
	}
	if err := writer.Flush(); err != nil {
		return "", errors.Wrap(err, "dumpDB Flush")
	}
	return buf.String(), nil
}

// entryExists checks if a Instance file for the given name exists in the dataFolder.
// Non-readOnly databases are named using a base58 encoding.
func entryExists(name string) error {
	name = strings.TrimSpace(name)
	fileName := filepath.Join(DataFolder, base58.FastBase58Encoding([]byte(name))+".sqlite3.db")
	if _, err := os.Stat(fileName); err != nil {
		return errors.Errorf("Instance '%s' does not exist", fileName)
	}
	return nil
}

// Drop closes and removes the underlying Instance file.
func (d *Instance) Drop() error {
	if err := d.CloseConnection(); err != nil {
		return errors.Wrap(err, "Drop CloseConnection")
	}
	slog.Info("Deleting Instance file", "path", d.Schema)
	if err := os.Remove(d.Schema); err != nil {
		return errors.Wrap(err, "Drop Remove")
	}
	return nil
}

// Open opens a SQLite3 Instance with the specified name.
// If readOnly is false, the Instance file is created (using a base58 encoded name) if it does not exist.
func Open(name string, readOnly bool) (*Instance, error) {
	name = strings.TrimSpace(name)
	d := &Instance{
		Schema: name,
	}

	if readOnly {
		d.Schema = filepath.Join(DataFolder, name)
	} else {
		encodedName := base58.FastBase58Encoding([]byte(name))
		d.Schema = filepath.Join(DataFolder, encodedName+".sqlite3.db")
	}

	// Determine if this is a new (non-readOnly) Instance.
	newDatabase := false
	if !readOnly {
		if _, err := os.Stat(d.Schema); os.IsNotExist(err) {
			newDatabase = true
		}
	}

	// Acquire a lock for the Instance file.
	for {
		dbLock.Lock()
		if _, locked := dbLock.Locked[d.Schema]; !locked {
			dbLock.Locked[d.Schema] = true
			dbLock.Unlock()
			break
		}
		dbLock.Unlock()
		time.Sleep(10 * time.Millisecond)
	}

	// Open the SQLite3 connection.
	var err error
	d.db, err = sql.Open("sqlite3", d.Schema)
	if err != nil {
		return nil, errors.Wrap(err, "Open sql.Open")
	}

	// If new, create a default table.
	if newDatabase {
		if err := d.CreateTables(false, "keystore", map[string]string{"value": "TEXT"}); err != nil {
			return nil, errors.Wrap(err, "Open CreateTables")
		}
		slog.Info("Created initial keystore table in new Instance")
	}

	return d, nil
}

// debugDB sets the logger's level based on the debugMode flag.
func (d *Instance) debugDB(debugMode bool) {
	if debugMode {
		slog.Info("debugDB")
	} else {
		slog.Info("debugDB")
	}
}

// CloseConnection terminates the Instance connection and releases the file lock.
func (d *Instance) CloseConnection() error {
	if d.isClosed {
		return nil
	}
	if err := d.db.Close(); err != nil {
		slog.Error(err.Error())
		return errors.Wrap(err, "CloseConnection db.CloseConnection")
	}
	dbLock.Lock()
	delete(dbLock.Locked, d.Schema)
	dbLock.Unlock()
	d.isClosed = true
	return nil
}

// Query executes a query that returns rows (e.g. SELECT).
func (d *Instance) Query(query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := d.db.Prepare(query)
	if err != nil {
		return nil, errors.Wrap(err, query)
	}
	defer stmt.Close()

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, errors.Wrap(err, query)
	}
	return rows, nil
}

// Exec executes a query without returning rows (e.g. INSERT, UPDATE, DELETE).
func (d *Instance) Exec(query string, args ...interface{}) error {
	if _, err := d.db.Exec(query, args...); err != nil {
		return errors.Wrap(err, "Exec")
	}
	return nil
}
