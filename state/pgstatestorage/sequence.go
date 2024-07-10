package pgstatestorage

import (
	"context"
	"fmt"

	"github.com/0xPolygonHermez/zkevm-aggregator/log"
	"github.com/0xPolygonHermez/zkevm-aggregator/state"
	"github.com/jackc/pgx/v4"
)

// AddSequence stores the sequence information to allow the aggregator verify sequences.
func (p *PostgresStorage) AddSequence(ctx context.Context, sequence state.Sequence, dbTx pgx.Tx) error {
	const addSequenceSQL = "INSERT INTO aggregator.sequence (from_batch_num, to_batch_num) VALUES($1, $2) ON CONFLICT (from_batch_num) DO UPDATE SET to_batch_num = $2"
	log.Info(fmt.Sprintf("zjg, addSequenceSQL:%v,%v,%v", addSequenceSQL, sequence.FromBatchNumber, sequence.ToBatchNumber))
	e := p.getExecQuerier(dbTx)
	_, err := e.Exec(ctx, addSequenceSQL, sequence.FromBatchNumber, sequence.ToBatchNumber)
	return err
}
