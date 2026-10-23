// Package backup creates, verifies, retains, and restores TrailLog SQLite
// backups.
//
// Live backups are created using SQLite's VACUUM INTO statement rather than
// copying the database file directly. Each completed backup is verified using
// SQLite integrity and foreign-key checks and is accompanied by a SHA-256
// checksum file.
package backup
