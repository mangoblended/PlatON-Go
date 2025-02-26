package snapshotdb

import (
	"errors"
	"math/big"

	"github.com/PlatONnetwork/PlatON-Go/common"
	"github.com/syndtr/goleveldb/leveldb/journal"
)

// fileType represent a file type.
type fileType int

// File types.
const (
	TypeCurrent fileType = 1 << iota
	TypeJournal
	TypeAll = TypeCurrent | TypeJournal
)

type journalData struct {
	Key, Value []byte
}

//
//const (
//	journalHeaderFromUnRecognized = "unRecognized"
//	journalHeaderFromRecognized   = "recognized"
//)

type journalHeader struct {
	ParentHash  common.Hash
	BlockNumber *big.Int `rlp:"nil"`
	//	From        string
	KvHash common.Hash
}

//
//func newJournalWriter(w io.WriteCloser) *journalWriter {
//	j := new(journalWriter)
//	j.writer = w
//	j.journal = journal.NewWriter(w)
//	return j
//}
/*//
//type journalWriter struct {
//	writer  io.WriteCloser
//	journal *journal.Writer
//}

func (j *journalWriter) Close() error {
	if err := j.journal.Close(); err != nil {
		return err
	}
	if err := j.writer.Close(); err != nil {
		return err
	}
	return nil
}*/

/*
func (s *snapshotDB) writeJournalHeader(blockNumber *big.Int, hash, parentHash common.Hash, comeFrom string) (*journalWriter, error) {
	fd := fileDesc{Type: TypeJournal, Num: blockNumber.Uint64(), BlockHash: hash}
	file, err := s.storage.Create(fd)
	if err != nil {
		return nil, err
	}
	writers := newJournalWriter(file)
	jHeader := journalHeader{
		ParentHash:  parentHash,
		BlockNumber: blockNumber,
		From:        comeFrom,
	}
	h, err := encode(jHeader)
	if err != nil {
		return nil, err
	}

	writer, err := writers.journal.Next()
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(h); err != nil {
		return nil, err
	}
	if err := writers.journal.Flush(); err != nil {
		return nil, err
	}
	return writers, nil
}*/

func (s *snapshotDB) rmJournalFile(blockNumber *big.Int, hash common.Hash) error {
	fd := fileDesc{Type: TypeJournal, Num: blockNumber.Uint64(), BlockHash: hash}
	return s.storage.Remove(fd)
}

func (s *snapshotDB) writeBlockToJournalAsynchronous(block *blockData) {
	s.journalSync.Add(1)
	go func(block *blockData) {
		if err := s.writeJournal(block); err != nil {
			logger.Error("Flush write Journal fail", "err", err, "block", block.Number, "hash", block.BlockHash.String())
		}
		s.journalSync.Done()
	}(block)
}

func (s *snapshotDB) writeJournal(block *blockData) error {

	fd := fileDesc{Type: TypeJournal, Num: block.Number.Uint64(), BlockHash: block.BlockHash}
	file, err := s.storage.Create(fd)
	if err != nil {
		return err
	}
	jwriters := journal.NewWriter(file)
	defer func() {
		err := jwriters.Close()
		if err != nil {
			logger.Error("write Journal fail for jwriters close", "num", block.Number, "err", err)
		}

		if err := file.Close(); err != nil {
			logger.Error("write Journal fail for file close", "num", block.Number, "err", err)
		}
	}()
	jHeader := journalHeader{
		ParentHash:  block.ParentHash,
		BlockNumber: block.Number,
		KvHash:      block.kvHash,
	}
	h, err := encode(jHeader)
	if err != nil {
		return err
	}
	writer, err := jwriters.Next()
	if err != nil {
		return err
	}
	if _, err := writer.Write(h); err != nil {
		return err
	}

	itr := block.data.NewIterator(nil)
	defer itr.Release()
	kvhash := common.ZeroHash
	for itr.Next() {
		toWrite, err := jwriters.Next()
		if err != nil {
			return errors.New("next err:" + err.Error())
		}
		key, val := common.CopyBytes(itr.Key()), common.CopyBytes(itr.Value())
		kvhash = s.generateKVHash(key, val, kvhash)
		jData := journalData{
			Key:   key,
			Value: val,
		}
		data, err := encode(jData)
		if _, err := toWrite.Write(data); err != nil {
			return err
		}
	}
	return nil
}
