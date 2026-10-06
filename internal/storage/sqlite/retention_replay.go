package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

var (
	ErrPurgedRecordCollision = errors.New("purged record provenance collision")
	ErrPurgedRecordState     = errors.New("invalid purged record provenance")
)

// These fingerprints survive the future deletion of raw/events. They retain
// physical identity and a content commitment, not an observation or an ACK
// inferred from a checkpoint. They have no automatic expiry.
const schemaV7 = `
CREATE TABLE purged_records (
    source_id TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    start_offset INTEGER NOT NULL CHECK(typeof(start_offset)='integer' AND start_offset>=0),
    end_offset INTEGER NOT NULL CHECK(typeof(end_offset)='integer' AND end_offset>start_offset),
    digest BLOB NOT NULL CHECK(typeof(digest)='blob' AND length(digest)=32),
    PRIMARY KEY(source_id,generation_id,start_offset),
    FOREIGN KEY(source_id,generation_id) REFERENCES file_generations(source_id,id)
) WITHOUT ROWID;
CREATE TRIGGER purged_records_immutable BEFORE UPDATE ON purged_records
BEGIN SELECT RAISE(ABORT,'immutable purged record provenance'); END;
`

func purgedRecordDigest(raw []byte, readError string) [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte("QueueAtlas/purged-record/v1\x00"))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(raw)))
	h.Write(size[:])
	h.Write(raw)
	binary.BigEndian.PutUint64(size[:], uint64(len(readError)))
	h.Write(size[:])
	h.Write([]byte(readError))
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

func checkPurgedRecord(ctx context.Context, tx *sql.Tx, sourceID string, record source.Record) (bool, error) {
	var end int64
	var digest []byte
	err := tx.QueryRowContext(ctx, `SELECT end_offset,digest FROM purged_records
		WHERE source_id=? AND generation_id=? AND start_offset=?`, sourceID, record.OriginID, record.Start).Scan(&end, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, ErrPurgedRecordState
	}
	if end <= record.Start || len(digest) != sha256.Size {
		return false, ErrPurgedRecordState
	}
	want := purgedRecordDigest(record.Raw, record.Error)
	if end != record.End || !bytes.Equal(digest, want[:]) {
		return false, ErrPurgedRecordCollision
	}
	return true, nil
}

// Only the future purge transaction may call this helper, before deleting the
// same raw record. The checkpoint must cover its exact physical origin. This
// prerequisite does not prove that the origin is complete or safe to purge.
func rememberPurgedRecord(ctx context.Context, tx *sql.Tx, rawID int64) error {
	var sourceID string
	var record source.Record
	var offset int64
	var anchor string
	err := tx.QueryRowContext(ctx, `SELECT r.source_id,r.generation_id,r.start_offset,r.end_offset,r.raw,r.read_error,c.offset,c.anchor_hash
		FROM raw_records r JOIN checkpoints c ON c.source_id=r.source_id AND c.generation_id=r.generation_id
		WHERE r.id=? AND r.end_offset<=c.offset AND c.anchor_hash<>''`, rawID).Scan(
		&sourceID, &record.OriginID, &record.Start, &record.End, &record.Raw, &record.Error, &offset, &anchor)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrPurgedRecordState
	}
	if sourceID == "" || record.OriginID == "" || record.Start < 0 || record.End <= record.Start || record.End > offset || anchor == "" || len(record.Raw) == 0 || len(record.Raw) > model.MaxLineBytes || int64(len(record.Raw)) > record.End-record.Start {
		return ErrPurgedRecordState
	}
	digest := purgedRecordDigest(record.Raw, record.Error)
	if _, err := tx.ExecContext(ctx, `INSERT INTO purged_records(source_id,generation_id,start_offset,end_offset,digest)
		VALUES(?,?,?,?,?) ON CONFLICT(source_id,generation_id,start_offset) DO NOTHING`,
		sourceID, record.OriginID, record.Start, record.End, digest[:]); err != nil {
		return err
	}
	_, err = checkPurgedRecord(ctx, tx, sourceID, record)
	return err
}
