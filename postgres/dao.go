package postgres

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	// Import postgres driver
	_ "github.com/lib/pq"
	"github.com/savsgio/gotils"
)

var dbRowPool = sync.Pool{
	New: func() interface{} {
		return new(DBRow)
	},
}

func acquireDBRow() *DBRow {
	return dbRowPool.Get().(*DBRow)
}

func releaseDBRow(row *DBRow) {
	row.Reset()
	dbRowPool.Put(row)
}

// Reset reset database row memory
func (row *DBRow) Reset() {
	row.sessionID = ""
	row.contents = ""
	row.lastActive = 0
}

// NewDao create new database access object
func NewDao(driver, dsn, tableName string) (*Dao, error) {
	db := &Dao{tableName: tableName}
	db.Driver = driver
	db.Dsn = dsn

	var err error
	db.Connection, err = sql.Open(db.Driver, db.Dsn)

	db.sqlGetSessionBySessionID = fmt.Sprintf("SELECT session_id,contents,last_active,expiration FROM %s WHERE session_id=$1", tableName)
	db.sqlCountSessions = fmt.Sprintf("SELECT count(*) as total FROM %s", tableName)
	db.sqlUpdateBySessionID = fmt.Sprintf("UPDATE %s SET contents=$1,last_active=$2,expiration=$3 WHERE session_id=$4", tableName)
	db.sqlDeleteBySessionID = fmt.Sprintf("DELETE FROM %s WHERE session_id=$1", tableName)
	db.sqlDeleteExpiredSessions = fmt.Sprintf("DELETE FROM %s WHERE last_active+expiration<=$1 AND expiration<>0", tableName)
	db.sqlInsert = fmt.Sprintf("INSERT INTO %s (session_id, contents, last_active, expiration) VALUES ($1,$2,$3,$4)", tableName)
	db.sqlRegenerate = fmt.Sprintf("UPDATE %s SET session_id=$1,last_active=$2,expiration=$3 WHERE session_id=$4", tableName)

	return db, err
}

// get session by sessionID
func (db *Dao) getSessionBySessionID(sessionID []byte) (*DBRow, error) {
	data := acquireDBRow()

	row, err := db.QueryRow(db.sqlGetSessionBySessionID, gotils.B2S(sessionID))
	if err != nil {
		return nil, err
	}

	err = row.Scan(&data.sessionID, &data.contents, &data.lastActive, &data.expiration)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	data.expiration *= time.Second

	return data, nil
}

// count sessions
func (db *Dao) countSessions() int {
	row, err := db.QueryRow(db.sqlCountSessions)
	if err != nil {
		return 0
	}

	var total int
	err = row.Scan(&total)
	if err != nil {
		return 0
	}

	return total
}

// update session by sessionID
func (db *Dao) updateBySessionID(sessionID, contents []byte, lastActiveTime int64, expiration time.Duration) (int64, error) {
	return db.Exec(db.sqlUpdateBySessionID, gotils.B2S(contents), lastActiveTime, expiration/time.Second, gotils.B2S(sessionID))
}

// delete session by sessionID
func (db *Dao) deleteBySessionID(sessionID []byte) (int64, error) {
	return db.Exec(db.sqlDeleteBySessionID, gotils.B2S(sessionID))
}

// delete session by expiration
func (db *Dao) deleteExpiredSessions() (int64, error) {
	return db.Exec(db.sqlDeleteExpiredSessions, time.Now().Unix())
}

// insert new session
func (db *Dao) insert(sessionID, contents []byte, lastActiveTime int64, expiration time.Duration) (int64, error) {
	return db.Exec(db.sqlInsert, gotils.B2S(sessionID), gotils.B2S(contents), lastActiveTime, expiration/time.Second)
}

// insert new session
func (db *Dao) regenerate(oldID, newID []byte, lastActiveTime int64, expiration time.Duration) (int64, error) {
	return db.Exec(db.sqlRegenerate, gotils.B2S(newID), lastActiveTime, expiration/time.Second, gotils.B2S(oldID))
}
