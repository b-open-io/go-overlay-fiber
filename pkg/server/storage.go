package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// SQLStorage implements the engine.Storage interface using SQL database
type SQLStorage struct {
	db *sql.DB
}

// NewSQLStorage creates a new SQL storage implementation
func NewSQLStorage(db *sql.DB) (*SQLStorage, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	storage := &SQLStorage{
		db: db,
	}

	// Initialize database schema
	if err := storage.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage schema: %w", err)
	}

	return storage, nil
}

// initializeSchema creates the necessary tables for overlay storage
func (s *SQLStorage) initializeSchema() error {
	// Create outputs table
	outputsTable := `
		CREATE TABLE IF NOT EXISTS overlay_outputs (
			outpoint_txid TEXT NOT NULL,
			outpoint_vout INTEGER NOT NULL,
			topic TEXT NOT NULL,
			satoshis INTEGER NOT NULL,
			script_data BLOB,
			spent BOOLEAN DEFAULT FALSE,
			spend_txid TEXT,
			block_height INTEGER,
			block_index INTEGER,
			beef_data BLOB,
			consumed_by TEXT,
			created_at REAL DEFAULT (julianday('now')),
			PRIMARY KEY (outpoint_txid, outpoint_vout, topic)
		)
	`

	if _, err := s.db.Exec(outputsTable); err != nil {
		return fmt.Errorf("failed to create outputs table: %w", err)
	}

	// Create applied transactions table
	appliedTxTable := `
		CREATE TABLE IF NOT EXISTS overlay_applied_transactions (
			txid TEXT NOT NULL,
			topic TEXT NOT NULL,
			created_at REAL DEFAULT (julianday('now')),
			PRIMARY KEY (txid, topic)
		)
	`

	if _, err := s.db.Exec(appliedTxTable); err != nil {
		return fmt.Errorf("failed to create applied transactions table: %w", err)
	}

	// Create interactions table for sync tracking
	interactionsTable := `
		CREATE TABLE IF NOT EXISTS overlay_interactions (
			host TEXT NOT NULL,
			topic TEXT NOT NULL,
			last_interaction REAL NOT NULL,
			PRIMARY KEY (host, topic)
		)
	`

	if _, err := s.db.Exec(interactionsTable); err != nil {
		return fmt.Errorf("failed to create interactions table: %w", err)
	}

	// Create indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_outputs_txid ON overlay_outputs(outpoint_txid)",
		"CREATE INDEX IF NOT EXISTS idx_outputs_topic ON overlay_outputs(topic)",
		"CREATE INDEX IF NOT EXISTS idx_outputs_spent ON overlay_outputs(spent)",
		"CREATE INDEX IF NOT EXISTS idx_outputs_created ON overlay_outputs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_interactions_last ON overlay_interactions(last_interaction)",
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// InsertOutput adds a new output to storage
func (s *SQLStorage) InsertOutput(ctx context.Context, utxo *engine.Output) error {
	query := `
		INSERT OR REPLACE INTO overlay_outputs 
		(outpoint_txid, outpoint_vout, topic, satoshis, script_data, spent, beef_data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, julianday('now'))
	`

	var scriptData []byte
	if utxo.Script != nil {
		scriptData = utxo.Script.Bytes()
	}

	_, err := s.db.ExecContext(ctx, query,
		utxo.Outpoint.Txid.String(),
		utxo.Outpoint.Index,
		utxo.Topic,
		utxo.Satoshis,
		scriptData,
		utxo.Spent,
		utxo.Beef,
	)

	return err
}

// FindOutput finds an output from storage
func (s *SQLStorage) FindOutput(ctx context.Context, outpoint *transaction.Outpoint, topic *string, spent *bool, includeBEEF bool) (*engine.Output, error) {
	query := `
		SELECT outpoint_txid, outpoint_vout, topic, satoshis, script_data, spent, spend_txid, 
		       block_height, block_index, consumed_by, beef_data, created_at
		FROM overlay_outputs 
		WHERE outpoint_txid = ? AND outpoint_vout = ?
	`
	args := []interface{}{outpoint.Txid.String(), outpoint.Index}

	if topic != nil {
		query += " AND topic = ?"
		args = append(args, *topic)
	}

	if spent != nil {
		query += " AND spent = ?"
		args = append(args, *spent)
	}

	query += " LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)

	return s.scanOutput(row, includeBEEF)
}

// FindOutputs finds multiple outputs from storage
func (s *SQLStorage) FindOutputs(ctx context.Context, outpoints []*transaction.Outpoint, topic string, spent *bool, includeBEEF bool) ([]*engine.Output, error) {
	if len(outpoints) == 0 {
		return []*engine.Output{}, nil
	}

	// Build IN clause for outpoints
	placeholders := make([]string, len(outpoints))
	args := make([]interface{}, 0, len(outpoints)*2+1)

	for i, outpoint := range outpoints {
		placeholders[i] = "(?, ?)"
		args = append(args, outpoint.Txid.String(), outpoint.Index)
	}

	query := fmt.Sprintf(`
		SELECT outpoint_txid, outpoint_vout, topic, satoshis, script_data, spent, spend_txid,
		       block_height, block_index, consumed_by, beef_data, created_at
		FROM overlay_outputs 
		WHERE (outpoint_txid, outpoint_vout) IN (%s) AND topic = ?
	`, fmt.Sprintf("(%s)", fmt.Sprintf("%s", placeholders[0])))

	// Add more placeholders if needed
	for i := 1; i < len(placeholders); i++ {
		query = fmt.Sprintf("%s OR (outpoint_txid, outpoint_vout) = %s", query, placeholders[i])
	}
	query += " AND topic = ?"
	args = append(args, topic)

	if spent != nil {
		query += " AND spent = ?"
		args = append(args, *spent)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outputs []*engine.Output
	for rows.Next() {
		output, err := s.scanOutputRow(rows, includeBEEF)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}

	return outputs, rows.Err()
}

// FindOutputsForTransaction finds outputs with a matching transaction ID from storage
func (s *SQLStorage) FindOutputsForTransaction(ctx context.Context, txid *chainhash.Hash, includeBEEF bool) ([]*engine.Output, error) {
	query := `
		SELECT outpoint_txid, outpoint_vout, topic, satoshis, script_data, spent, spend_txid,
		       block_height, block_index, consumed_by, beef_data, created_at
		FROM overlay_outputs 
		WHERE outpoint_txid = ?
	`

	rows, err := s.db.QueryContext(ctx, query, txid.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outputs []*engine.Output
	for rows.Next() {
		output, err := s.scanOutputRow(rows, includeBEEF)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}

	return outputs, rows.Err()
}

// FindUTXOsForTopic finds current UTXOs that have been admitted into a given topic
func (s *SQLStorage) FindUTXOsForTopic(ctx context.Context, topic string, since float64, limit uint32, includeBEEF bool) ([]*engine.Output, error) {
	query := `
		SELECT outpoint_txid, outpoint_vout, topic, satoshis, script_data, spent, spend_txid,
		       block_height, block_index, consumed_by, beef_data, created_at
		FROM overlay_outputs 
		WHERE topic = ? AND spent = FALSE AND created_at >= ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, topic, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outputs []*engine.Output
	for rows.Next() {
		output, err := s.scanOutputRow(rows, includeBEEF)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}

	return outputs, rows.Err()
}

// DeleteOutput deletes an output from storage
func (s *SQLStorage) DeleteOutput(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	query := `DELETE FROM overlay_outputs WHERE outpoint_txid = ? AND outpoint_vout = ? AND topic = ?`

	_, err := s.db.ExecContext(ctx, query, outpoint.Txid.String(), outpoint.Index, topic)
	return err
}

// MarkUTXOsAsSpent updates UTXOs as spent
func (s *SQLStorage) MarkUTXOsAsSpent(ctx context.Context, outpoints []*transaction.Outpoint, topic string, spendTxid *chainhash.Hash) error {
	if len(outpoints) == 0 {
		return nil
	}

	// Build transaction for atomic updates
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE overlay_outputs 
		SET spent = TRUE, spend_txid = ? 
		WHERE outpoint_txid = ? AND outpoint_vout = ? AND topic = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, outpoint := range outpoints {
		_, err = stmt.ExecContext(ctx, spendTxid.String(), outpoint.Txid.String(), outpoint.Index, topic)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateConsumedBy updates which outputs are consumed by this output
func (s *SQLStorage) UpdateConsumedBy(ctx context.Context, outpoint *transaction.Outpoint, topic string, consumedBy []*transaction.Outpoint) error {
	// Serialize consumed outpoints as JSON or comma-separated string
	var consumedByStr string
	if len(consumedBy) > 0 {
		for i, consumed := range consumedBy {
			if i > 0 {
				consumedByStr += ","
			}
			consumedByStr += fmt.Sprintf("%s:%d", consumed.Txid.String(), consumed.Index)
		}
	}

	query := `
		UPDATE overlay_outputs 
		SET consumed_by = ? 
		WHERE outpoint_txid = ? AND outpoint_vout = ? AND topic = ?
	`

	_, err := s.db.ExecContext(ctx, query, consumedByStr, outpoint.Txid.String(), outpoint.Index, topic)
	return err
}

// UpdateTransactionBEEF updates the beef data for a transaction
func (s *SQLStorage) UpdateTransactionBEEF(ctx context.Context, txid *chainhash.Hash, beef []byte) error {
	query := `UPDATE overlay_outputs SET beef_data = ? WHERE outpoint_txid = ?`

	_, err := s.db.ExecContext(ctx, query, beef, txid.String())
	return err
}

// UpdateOutputBlockHeight updates the block height on an output
func (s *SQLStorage) UpdateOutputBlockHeight(ctx context.Context, outpoint *transaction.Outpoint, topic string, blockHeight uint32, blockIndex uint64, ancillaryBeef []byte) error {
	query := `
		UPDATE overlay_outputs 
		SET block_height = ?, block_index = ?, beef_data = COALESCE(?, beef_data)
		WHERE outpoint_txid = ? AND outpoint_vout = ? AND topic = ?
	`

	_, err := s.db.ExecContext(ctx, query, blockHeight, blockIndex, ancillaryBeef, outpoint.Txid.String(), outpoint.Index, topic)
	return err
}

// InsertAppliedTransaction inserts record of the applied transaction
func (s *SQLStorage) InsertAppliedTransaction(ctx context.Context, tx *overlay.AppliedTransaction) error {
	query := `
		INSERT OR IGNORE INTO overlay_applied_transactions (txid, topic, created_at)
		VALUES (?, ?, julianday('now'))
	`

	_, err := s.db.ExecContext(ctx, query, tx.Txid.String(), tx.Topic)
	return err
}

// DoesAppliedTransactionExist checks if a duplicate transaction exists
func (s *SQLStorage) DoesAppliedTransactionExist(ctx context.Context, tx *overlay.AppliedTransaction) (bool, error) {
	query := `SELECT 1 FROM overlay_applied_transactions WHERE txid = ? AND topic = ? LIMIT 1`

	var exists int
	err := s.db.QueryRowContext(ctx, query, tx.Txid.String(), tx.Topic).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return exists == 1, nil
}

// UpdateLastInteraction updates the last interaction score for a given host and topic
func (s *SQLStorage) UpdateLastInteraction(ctx context.Context, host string, topic string, since float64) error {
	query := `
		INSERT OR REPLACE INTO overlay_interactions (host, topic, last_interaction)
		VALUES (?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query, host, topic, since)
	return err
}

// GetLastInteraction retrieves the last interaction score for a given host and topic
func (s *SQLStorage) GetLastInteraction(ctx context.Context, host string, topic string) (float64, error) {
	query := `SELECT last_interaction FROM overlay_interactions WHERE host = ? AND topic = ? LIMIT 1`

	var lastInteraction float64
	err := s.db.QueryRowContext(ctx, query, host, topic).Scan(&lastInteraction)
	if err == sql.ErrNoRows {
		return 0, nil // Return 0 if no record exists
	}
	if err != nil {
		return 0, err
	}

	return lastInteraction, nil
}

// Helper functions for scanning database rows

func (s *SQLStorage) scanOutput(row *sql.Row, includeBEEF bool) (*engine.Output, error) {
	var txidStr string
	var vout uint32
	var topic string
	var satoshis uint64
	var scriptData []byte
	var spent bool
	var spendTxidStr sql.NullString
	var blockHeight sql.NullInt32
	var blockIndex sql.NullInt64
	var consumedByStr sql.NullString
	var beefData []byte
	var createdAt float64

	err := row.Scan(&txidStr, &vout, &topic, &satoshis, &scriptData, &spent, &spendTxidStr,
		&blockHeight, &blockIndex, &consumedByStr, &beefData, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("output not found")
		}
		return nil, err
	}

	return s.buildOutput(txidStr, vout, topic, satoshis, scriptData, spent, spendTxidStr,
		blockHeight, blockIndex, consumedByStr, beefData, includeBEEF)
}

func (s *SQLStorage) scanOutputRow(rows *sql.Rows, includeBEEF bool) (*engine.Output, error) {
	var txidStr string
	var vout uint32
	var topic string
	var satoshis uint64
	var scriptData []byte
	var spent bool
	var spendTxidStr sql.NullString
	var blockHeight sql.NullInt32
	var blockIndex sql.NullInt64
	var consumedByStr sql.NullString
	var beefData []byte
	var createdAt float64

	err := rows.Scan(&txidStr, &vout, &topic, &satoshis, &scriptData, &spent, &spendTxidStr,
		&blockHeight, &blockIndex, &consumedByStr, &beefData, &createdAt)
	if err != nil {
		return nil, err
	}

	return s.buildOutput(txidStr, vout, topic, satoshis, scriptData, spent, spendTxidStr,
		blockHeight, blockIndex, consumedByStr, beefData, includeBEEF)
}

func (s *SQLStorage) buildOutput(txidStr string, vout uint32, topic string, satoshis uint64,
	scriptData []byte, spent bool, spendTxidStr sql.NullString, blockHeight sql.NullInt32,
	blockIndex sql.NullInt64, consumedByStr sql.NullString, beefData []byte, includeBEEF bool) (*engine.Output, error) {

	// Parse transaction ID
	txid, err := chainhash.NewHashFromHex(txidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction ID: %w", err)
	}

	output := &engine.Output{
		Outpoint: transaction.Outpoint{
			Txid:  *txid,
			Index: vout,
		},
		Topic:    topic,
		Satoshis: satoshis,
		Spent:    spent,
	}

	// Set locking script if present
	if len(scriptData) > 0 {
		output.Script = script.NewFromBytes(scriptData)
	}

	// Set block info if present
	if blockHeight.Valid {
		output.BlockHeight = uint32(blockHeight.Int32)
	}
	if blockIndex.Valid {
		output.BlockIdx = uint64(blockIndex.Int64)
	}

	// Parse consumed by outpoints if present
	if consumedByStr.Valid && consumedByStr.String != "" {
		consumedByParts := strings.Split(consumedByStr.String, ",")
		for _, part := range consumedByParts {
			if part = strings.TrimSpace(part); part != "" {
				txidVoutParts := strings.Split(part, ":")
				if len(txidVoutParts) == 2 {
					txid, err := chainhash.NewHashFromHex(txidVoutParts[0])
					if err != nil {
						continue // Skip invalid entries
					}
					vout, err := strconv.ParseUint(txidVoutParts[1], 10, 32)
					if err != nil {
						continue // Skip invalid entries
					}
					output.ConsumedBy = append(output.ConsumedBy, &transaction.Outpoint{
						Txid:  *txid,
						Index: uint32(vout),
					})
				}
			}
		}
	}

	// Include BEEF data if requested
	if includeBEEF && len(beefData) > 0 {
		output.Beef = beefData
	}

	return output, nil
}

// RunMigrations executes all pending database migrations on the storage
func (s *SQLStorage) RunMigrations(ctx context.Context, migrations []Migration, logger *log.Logger) error {
	if len(migrations) == 0 {
		logger.Printf("No migrations to run")
		return nil
	}

	// Create migrations table if it doesn't exist
	createMigrationsTable := `
		CREATE TABLE IF NOT EXISTS overlay_migrations (
			name TEXT PRIMARY KEY,
			executed_at REAL DEFAULT (julianday('now'))
		)
	`

	if _, err := s.db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Run each migration
	for _, migration := range migrations {
		// Check if migration has already been executed
		var exists bool
		checkQuery := "SELECT EXISTS(SELECT 1 FROM overlay_migrations WHERE name = ?)"
		err := s.db.QueryRowContext(ctx, checkQuery, migration.Name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migration.Name, err)
		}

		if exists {
			logger.Printf("Migration %s already executed, skipping", migration.Name)
			continue
		}

		logger.Printf("Running migration: %s", migration.Name)

		// Begin transaction for migration
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.Name, err)
		}

		// Execute migration
		err = migration.Up(s.db)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}

		// Record migration as executed
		_, err = tx.ExecContext(ctx, "INSERT INTO overlay_migrations (name) VALUES (?)", migration.Name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration.Name, err)
		}

		logger.Printf("Migration %s completed successfully", migration.Name)
	}

	logger.Printf("All migrations completed successfully")
	return nil
}
