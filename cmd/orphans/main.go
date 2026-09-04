// Command orphans finds node rows that no stored advert accounts for any more:
// nodes left behind by an observer delete that happened before the delete path
// swept them (see Store.purgeTargets). A node row is only ever created by a
// signature-valid advert, and observations are never pruned by retention, so a
// node with no surviving advert has nothing left that evidences it.
//
// Read-only by default: it prints what it would remove and exits. Pass -apply
// to delete the node rows, with the daemon stopped. A delete run first writes
// every row it is about to remove to a .sql file of INSERT statements, so a
// mistaken sweep can be put back: the node row is the only thing deleted, and
// re-inserting it restores it whole.
//
// Nodes carrying user-authored data (claim, note, private location, share) are
// never deleted — they are listed separately, the same rule the orphan pass in
// the store applies.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

type node struct {
	pubkey   string
	name     string
	lastSeen string
	adverts  int
	held     bool // has user-authored data
}

func main() {
	db := flag.String("db", "data/ridgeline.db", "path to ridgeline.db")
	apply := flag.Bool("apply", false, "delete the orphaned node rows (run with the daemon stopped)")
	backup := flag.String("backup", "orphans-restore.sql", "with -apply: file to write the removed rows to, as INSERTs")
	flag.Parse()

	// mode=ro for the survey; a delete run needs a writable handle.
	dsn := "file:" + *db
	if !*apply {
		dsn += "?mode=ro"
	}
	conn, err := sql.Open("sqlite", dsn)
	must(err)
	defer conn.Close()

	witnessed := scanAdverts(conn)
	fmt.Printf("adverts decoded: %d distinct node keys evidenced\n", len(witnessed))

	rows, err := conn.Query(`
		SELECT n.pubkey, COALESCE(n.name,''), COALESCE(n.last_seen,''), COALESCE(n.advert_count,0),
		       EXISTS(SELECT 1 FROM node_claims WHERE UPPER(node_pubkey) = UPPER(n.pubkey))
		    OR EXISTS(SELECT 1 FROM node_notes WHERE UPPER(node_pubkey) = UPPER(n.pubkey))
		    OR EXISTS(SELECT 1 FROM node_private_locations WHERE UPPER(node_pubkey) = UPPER(n.pubkey))
		    OR EXISTS(SELECT 1 FROM location_shares WHERE UPPER(node_pubkey) = UPPER(n.pubkey))
		FROM nodes n ORDER BY n.last_seen DESC`)
	must(err)
	var orphans, held []node
	total := 0
	for rows.Next() {
		var n node
		must(rows.Scan(&n.pubkey, &n.name, &n.lastSeen, &n.adverts, &n.held))
		total++
		if witnessed[strings.ToUpper(n.pubkey)] {
			continue
		}
		if n.held {
			held = append(held, n)
			continue
		}
		orphans = append(orphans, n)
	}
	rows.Close()
	must(rows.Err())

	fmt.Printf("node rows: %d total, %d unevidenced (%d of those held by user data)\n\n",
		total, len(orphans)+len(held), len(held))
	print("ORPHANED (would be deleted)", orphans)
	print("HELD BY USER DATA (kept)", held)

	if !*apply {
		fmt.Println("\ndry run — nothing was changed. Re-run with -apply to delete.")
		return
	}
	if len(orphans) == 0 {
		fmt.Println("\nnothing to delete")
		return
	}
	writeBackup(conn, orphans, *backup)
	fmt.Printf("\nwrote restore file: %s\n", *backup)

	var deleted int64
	for _, n := range orphans {
		res, err := conn.Exec(`DELETE FROM nodes WHERE pubkey = ?`, n.pubkey)
		must(err)
		c, _ := res.RowsAffected()
		deleted += c
	}
	fmt.Printf("deleted %d node rows\n", deleted)
}

// scanAdverts decodes every stored observation and returns the set of node keys
// that at least one surviving advert names. Every row is decoded rather than
// only payload_type='Advert': that column was classified at ingest by whatever
// decoder was current then, so trusting it would silently miss adverts stored
// under an older label — and missing one would delete a node that is fine.
func scanAdverts(conn *sql.DB) map[string]bool {
	rows, err := conn.Query(`SELECT raw_hex FROM observations`)
	must(err)
	defer rows.Close()

	witnessed := map[string]bool{}
	scanned := 0
	for rows.Next() {
		var raw string
		must(rows.Scan(&raw))
		scanned++
		if scanned%500000 == 0 {
			fmt.Fprintf(os.Stderr, "  scanned %d observations…\n", scanned)
		}
		pkt, err := meshcore.DecodeHex(raw)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		witnessed[strings.ToUpper(pkt.Advert.PublicKey)] = true
	}
	must(rows.Err())
	fmt.Printf("observations scanned: %d\n", scanned)
	return witnessed
}

// writeBackup dumps the full node rows about to be deleted as INSERT
// statements. Columns are read from the table itself so a later schema addition
// is carried without touching this tool.
func writeBackup(conn *sql.DB, ns []node, path string) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()

	cols := tableColumns(conn, "nodes")
	fmt.Fprintf(f, "-- ridgeline: %d node rows removed by cmd/orphans on %s\n",
		len(ns), time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(f, "-- restore with: sqlite3 ridgeline.db < %s\n", path)
	for _, n := range ns {
		row := conn.QueryRow(`SELECT * FROM nodes WHERE pubkey = ?`, n.pubkey)
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		must(row.Scan(ptrs...))
		lit := make([]string, len(cols))
		for i, v := range vals {
			lit[i] = sqlLiteral(v)
		}
		fmt.Fprintf(f, "INSERT INTO nodes (%s) VALUES (%s);\n",
			strings.Join(cols, ","), strings.Join(lit, ","))
	}
}

func tableColumns(conn *sql.DB, table string) []string {
	rows, err := conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
	must(err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		must(rows.Scan(&c))
		cols = append(cols, c)
	}
	must(rows.Err())
	return cols
}

func sqlLiteral(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%v", t)
	case []byte:
		return quote(string(t))
	case string:
		return quote(t)
	case time.Time:
		return quote(t.UTC().Format(time.RFC3339Nano))
	default:
		return quote(fmt.Sprintf("%v", t))
	}
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func print(title string, ns []node) {
	if len(ns) == 0 {
		return
	}
	fmt.Printf("%s — %d\n", title, len(ns))
	for _, n := range ns {
		name := n.name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  %s  %-28s last_seen=%s adverts=%d\n", n.pubkey[:12], trunc(name, 28), n.lastSeen, n.adverts)
	}
	fmt.Println()
}

func trunc(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
