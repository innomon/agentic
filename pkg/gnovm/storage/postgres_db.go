package storage

import (
	"errors"

	"github.com/gnolang/gno/tm2/pkg/db"
	"gorm.io/gorm"
)

// PostgresDB implements Gno's db.DB interface using GORM.
type PostgresDB struct {
	db    *gorm.DB
	table string
}

func NewPostgresDB(db *gorm.DB, table string) *PostgresDB {
	return &PostgresDB{db: db, table: table}
}

func (p *PostgresDB) Get(key []byte) ([]byte, error) {
	var fs FileSys
	if err := p.db.Table(p.table).Where("path = ?", string(key)).First(&fs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fs.Content, nil
}

func (p *PostgresDB) Has(key []byte) (bool, error) {
	var count int64
	if err := p.db.Table(p.table).Where("path = ?", string(key)).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (p *PostgresDB) Set(key, value []byte) error {
	fs := FileSys{
		Path:    string(key),
		Content: value,
	}
	return p.db.Table(p.table).Save(&fs).Error
}

func (p *PostgresDB) SetSync(key, value []byte) error {
	return p.Set(key, value)
}

func (p *PostgresDB) Delete(key []byte) error {
	return p.db.Table(p.table).Where("path = ?", string(key)).Delete(&FileSys{}).Error
}

func (p *PostgresDB) DeleteSync(key []byte) error {
	return p.Delete(key)
}

func (p *PostgresDB) Close() error {
	return nil
}

func (p *PostgresDB) Print() error {
	panic("not implemented")
}

func (p *PostgresDB) Stats() map[string]string {
	panic("not implemented")
}

func (p *PostgresDB) NewBatch() db.Batch {
	panic("not implemented")
}

func (p *PostgresDB) NewBatchWithSize(size int) db.Batch {
	return p.NewBatch()
}

func (p *PostgresDB) Iterator(start, end []byte) (db.Iterator, error) {
	panic("not implemented")
}

func (p *PostgresDB) ReverseIterator(start, end []byte) (db.Iterator, error) {
	panic("not implemented")
}

var _ db.DB = (*PostgresDB)(nil)
