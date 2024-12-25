package dao

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	var err error
	// https://briandouglas.ie/sqlite-defaults/
	db, err = sql.Open("sqlite", "file:foulbot.sqlite?"+
		"_journal_mode=WAL&"+
		"_synchronous=NORMAL&"+
		"_busy_timeout=5000&"+
		"_cache_size=-20000&"+
		"_foreign_keys=ON&"+
		"_auto_vacuum=INCREMENTAL&"+
		"_temp_store=MEMORY&"+
		"_mmap_size=2147483648&"+
		"_page_size=8192")
	if err != nil {
		panic(err)
	}
	makeTables()
}

func makeTables() {
	db.Exec(`
	    CREATE TABLE "polls" (
			"channel_id" TEXT NOT NULL,
    		"message_id" TEXT NOT NULL,
    		"points"	 INTEGER NOT NULL,
    		"reason"	 TEXT NOT NULL,
    		"expiry"	 TEXT NOT NULL,
    		"passed"	 INTEGER,
    		PRIMARY KEY("channel_id","message_id")
        );

		CREATE TABLE "gainers" (
			"channel_id" TEXT NOT NULL,
			"message_id" TEXT NOT NULL,
			"user_id"	 TEXT NOT NULL,
			PRIMARY KEY("channel_id","message_id"),
			FOREIGN KEY("channel_id") REFERENCES "polls"("channel_id"),
			FOREIGN KEY("message_id") REFERENCES "polls"("message_id")
		);

		CREATE TABLE "votes" (
			"channel_id" TEXT NOT NULL,
			"message_id" TEXT NOT NULL,
			"user_id"	 TEXT NOT NULL,
			"value"	INTEGER NOT NULL,
			PRIMARY KEY("channel_id","message_id"),
			FOREIGN KEY("channel_id") REFERENCES "polls"("channel_id"),
			FOREIGN KEY("message_id") REFERENCES "polls"("message_id")
		);
	`)
}
