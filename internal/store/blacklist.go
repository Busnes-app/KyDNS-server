package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) BlacklistSettings() (BlacklistSettings, error) { return blacklistSettings(s.db) }

func blacklistSettings(q queryer) (BlacklistSettings, error) {
	var b BlacklistSettings
	err := q.QueryRow(`SELECT enabled, block_ttl FROM blacklist_settings WHERE id = 1`).
		Scan(&b.Enabled, &b.BlockTTL)
	if errors.Is(err, sql.ErrNoRows) {
		return BlacklistSettings{Enabled: true, BlockTTL: 60}, nil
	}
	return b, err
}

func (s *Store) SetBlacklistSettings(b BlacklistSettings) error {
	_, err := s.db.Exec(`
		INSERT INTO blacklist_settings(id, enabled, block_ttl) VALUES(1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled,
		                              block_ttl = excluded.block_ttl`, b.Enabled, b.BlockTTL)
	return err
}

const listColumns = `id, name, url, format, description, enabled, builtin, interval_seconds,
	last_attempt_at, last_ok_at, last_error, etag, last_modified, entry_count, skipped_count`

// scanList reads the metadata columns. body is the snapshot text, or "" where
// the caller did not select it.
func scanList(sc interface{ Scan(...any) error }, body *string) (BlacklistList, error) {
	var l BlacklistList
	dest := []any{
		&l.ID, &l.Name, &l.URL, &l.Format, &l.Description, &l.Enabled, &l.Builtin,
		&l.IntervalSeconds, &l.LastAttemptAt, &l.LastOKAt, &l.LastError,
		&l.ETag, &l.LastModified, &l.EntryCount, &l.SkippedCount,
	}
	if body != nil {
		dest = append(dest, body)
	}
	if err := sc.Scan(dest...); err != nil {
		return BlacklistList{}, err
	}
	if body != nil && *body != "" {
		l.Snapshot = strings.Split(*body, "\n")
	}
	return l, nil
}

// BlacklistLists loads every list with its snapshot. The policy rebuild is the
// caller; the UI uses BlacklistListMetas instead.
func (s *Store) BlacklistLists() ([]BlacklistList, error) {
	return blacklistLists(s.db, true)
}

// BlacklistListMetas loads every list without its snapshot, so rendering a
// screen does not pull megabytes of domains out of the database.
func (s *Store) BlacklistListMetas() ([]BlacklistList, error) {
	return blacklistLists(s.db, false)
}

func blacklistLists(db queryer, withBody bool) ([]BlacklistList, error) {
	q := `SELECT ` + listColumns
	if withBody {
		q += `, snapshot`
	}
	q += ` FROM blacklist_lists ORDER BY builtin DESC, name`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlacklistList{}
	for rows.Next() {
		var body string
		var p *string
		if withBody {
			p = &body
		}
		l, err := scanList(rows, p)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) BlacklistListByID(id int64) (BlacklistList, error) {
	var body string
	l, err := scanList(s.db.QueryRow(`SELECT `+listColumns+`, snapshot FROM blacklist_lists WHERE id = ?`, id), &body)
	if errors.Is(err, sql.ErrNoRows) {
		return BlacklistList{}, fmt.Errorf("%w: blacklist list %d", ErrNotFound, id)
	}
	return l, err
}

// PutBlacklistList writes a list definition. It deliberately never touches the
// snapshot or the refresh metadata: editing a name must not throw away a
// working download.
func (s *Store) PutBlacklistList(l BlacklistList) (int64, error) {
	if l.ID == 0 {
		res, err := s.db.Exec(`
			INSERT INTO blacklist_lists(name, url, format, description, enabled, builtin, interval_seconds)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			l.Name, l.URL, l.Format, l.Description, l.Enabled, l.Builtin, l.IntervalSeconds)
		if err != nil {
			if isUnique(err, "blacklist_lists.name") {
				return 0, fmt.Errorf("%w: list %s", ErrDuplicateName, l.Name)
			}
			return 0, err
		}
		return res.LastInsertId()
	}
	// A new source or format drops the cache validators, so the next fetch is
	// unconditional: a 304 sent against the old source's ETag or date would
	// keep the old body (or its old parse) forever. It also makes the list due
	// now rather than a whole interval later. The body stays until replaced.
	res, err := s.db.Exec(`
		UPDATE blacklist_lists
		SET name = ?, url = ?, format = ?, description = ?, enabled = ?, interval_seconds = ?,
		    etag = CASE WHEN url = ? AND format = ? THEN etag ELSE '' END,
		    last_modified = CASE WHEN url = ? AND format = ? THEN last_modified ELSE '' END,
		    last_attempt_at = CASE WHEN url = ? AND format = ? THEN last_attempt_at ELSE 0 END
		WHERE id = ?`,
		l.Name, l.URL, l.Format, l.Description, l.Enabled, l.IntervalSeconds,
		l.URL, l.Format, l.URL, l.Format, l.URL, l.Format, l.ID)
	if err != nil {
		if isUnique(err, "blacklist_lists.name") {
			return 0, fmt.Errorf("%w: list %s", ErrDuplicateName, l.Name)
		}
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, fmt.Errorf("%w: blacklist list %d", ErrNotFound, l.ID)
	}
	return l.ID, nil
}

// SetBlacklistSnapshot replaces a list's body in one statement, which is what
// makes a refresh transactional: readers see the old body or the new one.
func (s *Store) SetBlacklistSnapshot(id int64, domains []string, skipped int, etag, lastModified string, at int64) error {
	return s.InstallBlacklistFetch(BlacklistList{ID: id}, domains, skipped, etag, lastModified, at)
}

// InstallBlacklistFetch is SetBlacklistSnapshot for a download of l: it lands
// only while the row still has the URL and format l was fetched with, so a list
// edited (or replaced under the same ID) during the download keeps its own
// source. An empty URL skips that check.
func (s *Store) InstallBlacklistFetch(l BlacklistList, domains []string, skipped int, etag, lastModified string, at int64) error {
	res, err := s.db.Exec(`
		UPDATE blacklist_lists
		SET snapshot = ?, entry_count = ?, skipped_count = ?, etag = ?, last_modified = ?,
		    last_ok_at = ?, last_attempt_at = ?, last_error = ''
		WHERE id = ? AND (? = '' OR (url = ? AND format = ?))`,
		strings.Join(domains, "\n"), len(domains), skipped, etag, lastModified, at, at, l.ID, l.URL, l.URL, l.Format)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: blacklist list %d with that source", ErrNotFound, l.ID)
	}
	return nil
}

// SetBlacklistError records a failed refresh of l's source. The snapshot is
// untouched, so the last known-good data keeps serving. Like
// InstallBlacklistFetch, it writes nothing once the row has another source:
// the edit made that one due now, and an old failure must not postpone it.
func (s *Store) SetBlacklistError(l BlacklistList, msg string, at int64) error {
	_, err := s.db.Exec(
		`UPDATE blacklist_lists SET last_error = ?, last_attempt_at = ?
		WHERE id = ? AND (? = '' OR (url = ? AND format = ?))`, msg, at, l.ID, l.URL, l.URL, l.Format)
	return err
}

// TouchBlacklistAttempt records a refresh of l's source that succeeded with no
// new content, so a 304 does not look like a list that has stopped updating.
// Nothing is written once the row has another source.
func (s *Store) TouchBlacklistAttempt(l BlacklistList, at int64) error {
	_, err := s.db.Exec(
		`UPDATE blacklist_lists SET last_attempt_at = ?, last_ok_at = ?, last_error = ''
		WHERE id = ? AND (? = '' OR (url = ? AND format = ?))`, at, at, l.ID, l.URL, l.URL, l.Format)
	return err
}

func (s *Store) DeleteBlacklistList(id int64) error {
	res, err := s.db.Exec(`DELETE FROM blacklist_lists WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: blacklist list %d", ErrNotFound, id)
	}
	return nil
}

func (s *Store) BlacklistRules() ([]BlacklistRule, error) { return blacklistRules(s.db) }

func blacklistRules(q queryer) ([]BlacklistRule, error) {
	rows, err := q.Query(`SELECT id, kind, domain FROM blacklist_rules ORDER BY kind, domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlacklistRule{}
	for rows.Next() {
		var r BlacklistRule
		if err := rows.Scan(&r.ID, &r.Kind, &r.Domain); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) PutBlacklistRule(r BlacklistRule) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO blacklist_rules(kind, domain) VALUES(?, ?)`, r.Kind, r.Domain)
	if err != nil {
		if isUnique(err, "blacklist_rules.domain") {
			return 0, fmt.Errorf("%w: a rule for %s already exists", ErrDuplicateName, r.Domain)
		}
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteBlacklistRule(id int64) error {
	res, err := s.db.Exec(`DELETE FROM blacklist_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: blacklist rule %d", ErrNotFound, id)
	}
	return nil
}

// replaceBlacklistDefinitions writes a snapshot's blacklist policy against an
// already-open transaction. Lists are matched to existing rows by name so a
// locally downloaded body survives; only the definition columns are written.
func replaceBlacklistDefinitions(tx *sql.Tx, set BlacklistSettings, lists []BlacklistList, rules []BlacklistRule) error {
	if _, err := tx.Exec(`
		INSERT INTO blacklist_settings(id, enabled, block_ttl) VALUES(1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled,
		                              block_ttl = excluded.block_ttl`, set.Enabled, set.BlockTTL); err != nil {
		return err
	}

	existing := map[string]string{} // name -> url
	rows, err := tx.Query(`SELECT name, url FROM blacklist_lists`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, url string
		if err := rows.Scan(&name, &url); err != nil {
			rows.Close()
			return err
		}
		existing[name] = url
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	incoming := map[string]bool{}
	for _, l := range lists {
		incoming[l.Name] = true
		if oldURL, ok := existing[l.Name]; ok {
			// The body belongs to the URL, not the name: a repointed list must
			// not keep serving the old source's downloaded content or ETag.
			if oldURL != l.URL {
				if _, err := tx.Exec(`
					UPDATE blacklist_lists
					SET url = ?, format = ?, description = ?, enabled = ?, builtin = ?, interval_seconds = ?,
					    snapshot = '', entry_count = 0, skipped_count = 0, etag = '', last_modified = '',
					    last_attempt_at = 0, last_ok_at = 0, last_error = ''
					WHERE name = ?`,
					l.URL, l.Format, l.Description, l.Enabled, l.Builtin, l.IntervalSeconds, l.Name); err != nil {
					return err
				}
				continue
			}
			// Same URL, new format: the body is kept, but the validators go and
			// the list is due, so the next fetch reparses rather than getting a 304.
			if _, err := tx.Exec(`
				UPDATE blacklist_lists
				SET url = ?, format = ?, description = ?, enabled = ?, builtin = ?, interval_seconds = ?,
				    etag = CASE WHEN format = ? THEN etag ELSE '' END,
				    last_modified = CASE WHEN format = ? THEN last_modified ELSE '' END,
				    last_attempt_at = CASE WHEN format = ? THEN last_attempt_at ELSE 0 END
				WHERE name = ?`,
				l.URL, l.Format, l.Description, l.Enabled, l.Builtin, l.IntervalSeconds,
				l.Format, l.Format, l.Format, l.Name); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO blacklist_lists(name, url, format, description, enabled, builtin, interval_seconds)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			l.Name, l.URL, l.Format, l.Description, l.Enabled, l.Builtin, l.IntervalSeconds); err != nil {
			return err
		}
	}
	for name := range existing {
		if !incoming[name] {
			if _, err := tx.Exec(`DELETE FROM blacklist_lists WHERE name = ?`, name); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(`DELETE FROM blacklist_rules`); err != nil {
		return err
	}
	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO blacklist_rules(kind, domain) VALUES(?, ?)`, r.Kind, r.Domain); err != nil {
			if isUnique(err, "blacklist_rules.domain") {
				return fmt.Errorf("%w: a rule for %s already exists", ErrDuplicateName, r.Domain)
			}
			return err
		}
	}
	return nil
}

// ReplaceBlacklist writes a whole imported policy in one transaction. A list
// whose URL survives the import keeps its downloaded body, so restoring a
// backup does not force every source to re-download.
func (s *Store) ReplaceBlacklist(set BlacklistSettings, lists []BlacklistList, rules []BlacklistRule) error {
	type body struct {
		snapshot, format         string
		etag, lastModified       string
		entryCount, skippedCount int
		lastOKAt, lastAttemptAt  int64
	}
	kept := map[string]body{}
	rows, err := s.db.Query(`SELECT url, format, snapshot, etag, last_modified, entry_count, skipped_count, last_ok_at, last_attempt_at FROM blacklist_lists`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var url string
		var b body
		if err := rows.Scan(&url, &b.format, &b.snapshot, &b.etag, &b.lastModified,
			&b.entryCount, &b.skippedCount, &b.lastOKAt, &b.lastAttemptAt); err != nil {
			rows.Close()
			return err
		}
		kept[url] = b
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`DELETE FROM blacklist_rules`, `DELETE FROM blacklist_lists`} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO blacklist_settings(id, enabled, block_ttl) VALUES(1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled,
		                              block_ttl = excluded.block_ttl`, set.Enabled, set.BlockTTL); err != nil {
		return err
	}
	for _, l := range lists {
		b := kept[l.URL]
		// A new format keeps the body until the next fetch, which must reparse.
		if b.format != l.Format {
			b.etag, b.lastModified, b.lastAttemptAt = "", "", 0
		}
		if _, err := tx.Exec(`
			INSERT INTO blacklist_lists(name, url, format, description, enabled, builtin, interval_seconds,
			  snapshot, etag, last_modified, entry_count, skipped_count, last_ok_at, last_attempt_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.Name, l.URL, l.Format, l.Description, l.Enabled, l.Builtin, l.IntervalSeconds,
			b.snapshot, b.etag, b.lastModified, b.entryCount, b.skippedCount,
			b.lastOKAt, b.lastAttemptAt); err != nil {
			if isUnique(err, "blacklist_lists.name") {
				return fmt.Errorf("%w: list %s", ErrDuplicateName, l.Name)
			}
			return err
		}
	}
	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO blacklist_rules(kind, domain) VALUES(?, ?)`, r.Kind, r.Domain); err != nil {
			if isUnique(err, "blacklist_rules.domain") {
				return fmt.Errorf("%w: a rule for %s already exists", ErrDuplicateName, r.Domain)
			}
			return err
		}
	}
	return tx.Commit()
}
