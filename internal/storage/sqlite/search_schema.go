package sqlite

// Preserve earlier schemas and indexes. Message-ID is a literal search value,
// never an identity or uniqueness constraint; repeated IDs remain observable.
const schemaV5 = `
CREATE INDEX events_message_id_time ON events(message_id, time_utc_ns, id) WHERE message_id IS NOT NULL;
`
