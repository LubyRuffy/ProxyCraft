package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS traffic_entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	start_time INTEGER NOT NULL,
	end_time INTEGER,
	duration INTEGER,
	host TEXT,
	host_with_schema TEXT,
	method TEXT,
	schema TEXT,
	protocol TEXT,
	url TEXT,
	path TEXT,
	status_code INTEGER,
	content_type TEXT,
	content_size INTEGER,
	is_sse INTEGER,
	is_sse_completed INTEGER,
	is_https INTEGER,
	process_name TEXT,
	process_icon TEXT,
	request_body BLOB,
	response_body BLOB,
	request_headers BLOB,
	response_headers BLOB,
	error TEXT
);
`

func (h *WebHandler) initSQLite(dbPath string) error {
	if dbPath == "" {
		dbPath = "proxycraft.db"
	}

	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_traffic_entries_id ON traffic_entries(id);"); err != nil {
		_ = db.Close()
		return err
	}

	h.db = db
	h.dbPath = dbPath
	return nil
}

func (h *WebHandler) insertEntry(entry *TrafficEntry) (string, error) {
	if h.db == nil {
		return "", errors.New("sqlite not initialized")
	}

	requestHeaders, err := marshalHeaders(entry.RequestHeaders)
	if err != nil {
		return "", err
	}

	result, err := h.db.Exec(
		`INSERT INTO traffic_entries (
			start_time, host, host_with_schema, method, schema, protocol, url, path,
			is_sse, is_sse_completed, is_https, process_name, process_icon, request_body, request_headers
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)` ,
		toMillis(entry.StartTime),
		emptyToNil(entry.Host),
		emptyToNil(entry.HostWithSchema),
		emptyToNil(entry.Method),
		emptyToNil(entry.Schema),
		emptyToNil(entry.Protocol),
		emptyToNil(entry.URL),
		emptyToNil(entry.Path),
		boolToInt(entry.IsSSE),
		boolToInt(entry.IsSSECompleted),
		boolToInt(entry.IsHTTPS),
		emptyToNil(entry.ProcessName),
		emptyToNil(entry.ProcessIcon),
		emptyBytesToNil(entry.RequestBody),
		emptyBytesToNil(requestHeaders),
	)
	if err != nil {
		return "", err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(id, 10), nil
}

func (h *WebHandler) updateResponse(entry *TrafficEntry) error {
	if h.db == nil {
		return nil
	}

	responseHeaders, err := marshalHeaders(entry.ResponseHeaders)
	if err != nil {
		return err
	}

	_, err = h.db.Exec(
		`UPDATE traffic_entries SET
			end_time = ?,
			duration = ?,
			status_code = ?,
			content_type = ?,
			content_size = ?,
			is_sse = ?,
			is_sse_completed = ?,
			is_https = ?,
			response_headers = ?,
			response_body = ?
		WHERE id = ?`,
		toNullableMillis(entry.EndTime),
		entry.Duration,
		entry.StatusCode,
		emptyToNil(entry.ContentType),
		entry.ContentSize,
		boolToInt(entry.IsSSE),
		boolToInt(entry.IsSSECompleted),
		boolToInt(entry.IsHTTPS),
		emptyBytesToNil(responseHeaders),
		emptyBytesToNil(entry.ResponseBody),
		entry.ID,
	)
	return err
}

func (h *WebHandler) updateError(entry *TrafficEntry) error {
	if h.db == nil {
		return nil
	}

	_, err := h.db.Exec(
		`UPDATE traffic_entries SET end_time = ?, duration = ?, error = ? WHERE id = ?`,
		toNullableMillis(entry.EndTime),
		entry.Duration,
		emptyToNil(entry.Error),
		entry.ID,
	)
	return err
}

func (h *WebHandler) updateSSE(entry *TrafficEntry) error {
	if h.db == nil {
		return nil
	}

	_, err := h.db.Exec(
		`UPDATE traffic_entries SET
			end_time = ?,
			duration = ?,
			content_type = ?,
			content_size = ?,
			response_body = ?,
			is_sse_completed = ?
		WHERE id = ?`,
		toNullableMillis(entry.EndTime),
		entry.Duration,
		emptyToNil(entry.ContentType),
		entry.ContentSize,
		emptyBytesToNil(entry.ResponseBody),
		boolToInt(entry.IsSSECompleted),
		entry.ID,
	)
	return err
}

func (h *WebHandler) loadEntries(limit int) ([]*TrafficEntry, error) {
	if h.db == nil {
		return []*TrafficEntry{}, nil
	}

	rows, err := h.db.Query(
		`SELECT id, start_time, end_time, duration, host, host_with_schema, method, schema, protocol, url, path,
			status_code, content_type, content_size, is_sse, is_sse_completed, is_https, process_name, process_icon, error
		FROM traffic_entries ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]*TrafficEntry, 0)
	for rows.Next() {
		entry, err := scanEntryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	reverseEntries(entries)
	return entries, nil
}

func (h *WebHandler) loadEntriesAfterID(offsetID string) ([]*TrafficEntry, error) {
	if offsetID == "" {
		return h.loadEntries(1000)
	}

	offsetValue, err := strconv.ParseInt(offsetID, 10, 64)
	if err != nil {
		return h.loadEntries(1000)
	}

	var exists int
	if err := h.db.QueryRow("SELECT 1 FROM traffic_entries WHERE id = ? LIMIT 1", offsetValue).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return h.loadEntries(1000)
		}
		return nil, err
	}

	rows, err := h.db.Query(
		`SELECT id, start_time, end_time, duration, host, host_with_schema, method, schema, protocol, url, path,
			status_code, content_type, content_size, is_sse, is_sse_completed, is_https, process_name, process_icon, error
		FROM traffic_entries WHERE id > ? ORDER BY id ASC`,
		offsetValue,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]*TrafficEntry, 0)
	for rows.Next() {
		entry, err := scanEntryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func (h *WebHandler) loadEntry(id string) (*TrafficEntry, error) {
	if h.db == nil {
		return nil, nil
	}

	row := h.db.QueryRow(
		`SELECT id, start_time, end_time, duration, host, host_with_schema, method, schema, protocol, url, path,
			status_code, content_type, content_size, is_sse, is_sse_completed, is_https, process_name, process_icon,
			request_body, response_body, request_headers, response_headers, error
		FROM traffic_entries WHERE id = ?`,
		id,
	)

	var (
		entryID           int64
		startTime         int64
		endTime           sql.NullInt64
		duration          sql.NullInt64
		host              sql.NullString
		hostWithSchema    sql.NullString
		method            sql.NullString
		schema            sql.NullString
		protocol          sql.NullString
		url               sql.NullString
		path              sql.NullString
		statusCode        sql.NullInt64
		contentType       sql.NullString
		contentSize       sql.NullInt64
		isSSE             sql.NullInt64
		isSSECompleted    sql.NullInt64
		isHTTPS           sql.NullInt64
		processName       sql.NullString
		processIcon       sql.NullString
		requestBody       []byte
		responseBody      []byte
		requestHeadersRaw []byte
		responseHeadersRaw []byte
		errorMsg          sql.NullString
	)

	if err := row.Scan(
		&entryID,
		&startTime,
		&endTime,
		&duration,
		&host,
		&hostWithSchema,
		&method,
		&schema,
		&protocol,
		&url,
		&path,
		&statusCode,
		&contentType,
		&contentSize,
		&isSSE,
		&isSSECompleted,
		&isHTTPS,
		&processName,
		&processIcon,
		&requestBody,
		&responseBody,
		&requestHeadersRaw,
		&responseHeadersRaw,
		&errorMsg,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	entry := buildEntryFromRow(
		entryID,
		startTime,
		endTime,
		duration,
		host,
		hostWithSchema,
		method,
		schema,
		protocol,
		url,
		path,
		statusCode,
		contentType,
		contentSize,
		isSSE,
		isSSECompleted,
		isHTTPS,
		processName,
		processIcon,
		errorMsg,
	)

	entry.RequestBody = requestBody
	entry.ResponseBody = responseBody
	if headers, err := unmarshalHeaders(requestHeadersRaw); err == nil {
		entry.RequestHeaders = headers
	}
	if headers, err := unmarshalHeaders(responseHeadersRaw); err == nil {
		entry.ResponseHeaders = headers
	}

	return entry, nil
}

func (h *WebHandler) clearEntriesInDB() error {
	if h.db == nil {
		return nil
	}
	_, err := h.db.Exec("DELETE FROM traffic_entries")
	return err
}

func (h *WebHandler) cleanupOldEntries() {
	if h.db == nil {
		return
	}

	var total int
	if err := h.db.QueryRow("SELECT COUNT(*) FROM traffic_entries").Scan(&total); err != nil {
		return
	}
	if total <= h.maxEntries {
		return
	}

	deleteCount := total - h.maxEntries
	if h.verbose {
		log.Printf("[WebHandler] 清理 %d 条旧流量记录，当前总数: %d", deleteCount, total)
	}

	_, _ = h.db.Exec(
		"DELETE FROM traffic_entries WHERE id IN (SELECT id FROM traffic_entries WHERE end_time IS NOT NULL ORDER BY id ASC LIMIT ?)",
		deleteCount,
	)

	if err := h.db.QueryRow("SELECT COUNT(*) FROM traffic_entries").Scan(&total); err != nil {
		return
	}
	if total <= h.maxEntries {
		return
	}
	deleteCount = total - h.maxEntries
	_, _ = h.db.Exec(
		"DELETE FROM traffic_entries WHERE id IN (SELECT id FROM traffic_entries ORDER BY id ASC LIMIT ?)",
		deleteCount,
	)
}

func scanEntryRow(rows *sql.Rows) (*TrafficEntry, error) {
	var (
		entryID        int64
		startTime      int64
		endTime        sql.NullInt64
		duration       sql.NullInt64
		host           sql.NullString
		hostWithSchema sql.NullString
		method         sql.NullString
		schema         sql.NullString
		protocol       sql.NullString
		url            sql.NullString
		path           sql.NullString
		statusCode     sql.NullInt64
		contentType    sql.NullString
		contentSize    sql.NullInt64
		isSSE          sql.NullInt64
		isSSECompleted sql.NullInt64
		isHTTPS        sql.NullInt64
		processName    sql.NullString
		processIcon    sql.NullString
		errorMsg       sql.NullString
	)

	if err := rows.Scan(
		&entryID,
		&startTime,
		&endTime,
		&duration,
		&host,
		&hostWithSchema,
		&method,
		&schema,
		&protocol,
		&url,
		&path,
		&statusCode,
		&contentType,
		&contentSize,
		&isSSE,
		&isSSECompleted,
		&isHTTPS,
		&processName,
		&processIcon,
		&errorMsg,
	); err != nil {
		return nil, err
	}

	return buildEntryFromRow(
		entryID,
		startTime,
		endTime,
		duration,
		host,
		hostWithSchema,
		method,
		schema,
		protocol,
		url,
		path,
		statusCode,
		contentType,
		contentSize,
		isSSE,
		isSSECompleted,
		isHTTPS,
		processName,
		processIcon,
		errorMsg,
	), nil
}

func buildEntryFromRow(
	entryID int64,
	startTime int64,
	endTime sql.NullInt64,
	duration sql.NullInt64,
	host sql.NullString,
	hostWithSchema sql.NullString,
	method sql.NullString,
	schema sql.NullString,
	protocol sql.NullString,
	url sql.NullString,
	path sql.NullString,
	statusCode sql.NullInt64,
	contentType sql.NullString,
	contentSize sql.NullInt64,
	isSSE sql.NullInt64,
	isSSECompleted sql.NullInt64,
	isHTTPS sql.NullInt64,
	processName sql.NullString,
	processIcon sql.NullString,
	errorMsg sql.NullString,
) *TrafficEntry {
	entry := &TrafficEntry{
		ID:             strconv.FormatInt(entryID, 10),
		StartTime:      time.Unix(0, startTime*int64(time.Millisecond)),
		Host:           host.String,
		HostWithSchema: hostWithSchema.String,
		Method:         method.String,
		Schema:         schema.String,
		Protocol:       protocol.String,
		URL:            url.String,
		Path:           path.String,
		StatusCode:     int(statusCode.Int64),
		ContentType:    contentType.String,
		ContentSize:    int(contentSize.Int64),
		IsSSE:          isSSE.Int64 == 1,
		IsSSECompleted: isSSECompleted.Int64 == 1,
		IsHTTPS:        isHTTPS.Int64 == 1,
		ProcessName:    processName.String,
		ProcessIcon:    processIcon.String,
		Error:          errorMsg.String,
	}

	if endTime.Valid && endTime.Int64 > 0 {
		entry.EndTime = time.Unix(0, endTime.Int64*int64(time.Millisecond))
	}
	if duration.Valid {
		entry.Duration = duration.Int64
	}

	return entry
}

func marshalHeaders(headers map[string][]string) ([]byte, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	return json.Marshal(headers)
}

func unmarshalHeaders(data []byte) (map[string][]string, error) {
	if len(data) == 0 {
		return map[string][]string{}, nil
	}

	var headers map[string][]string
	if err := json.Unmarshal(data, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func toMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func toNullableMillis(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.UnixNano() / int64(time.Millisecond)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func emptyToNil(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func emptyBytesToNil(value []byte) interface{} {
	if len(value) == 0 {
		return nil
	}
	return value
}

func reverseEntries(entries []*TrafficEntry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}
